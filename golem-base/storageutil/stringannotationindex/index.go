// Package stringannotationindex defines a data structure to efficiently query string annotations
package stringannotationindex

// TODO:
// * add lexicographical sort

import (
	"bytes"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/bytekeyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset/array"
	"github.com/holiman/uint256"
	"golang.org/x/text/unicode/norm"
)

var entitiesSetKey = []byte("golemBase.stringannotationindex.entities")
var suffixEntitiesKey = []byte("golemBase.stringannotationindex.entities.suffix")
var childrenKey = []byte("golemBase.stringannotationindex.characters")
var prefixesKey = []byte("golemBase.stringannotationindex.prefix")

type node struct {
	ix     *Index
	path   []byte
	prefix []byte
}

func findCommonPrefix(s1 []byte, s2 []byte) []byte {
	prefix := bytes.Buffer{}
	for i := 0; i < min(len(s1), len(s2)) && s1[i] == s2[i]; i++ {
		prefix.WriteByte(s1[i])
	}
	return prefix.Bytes()
}

func (ix *Index) getEntityKeySetAddress(fullPath []byte) common.Hash {
	return crypto.Keccak256Hash(
		entitiesSetKey,
		ix.annotationKey,
		fullPath,
	)
}

func (n *node) getEntityKeySetAddress() common.Hash {
	return n.ix.getEntityKeySetAddress(slices.Concat(n.path, n.prefix))
}

func (n *node) getSuffixEntityArrayAddress() common.Hash {
	return crypto.Keccak256Hash(
		suffixEntitiesKey,
		n.ix.annotationKey,
		n.path,
		n.prefix,
	)
}

func (n *node) getChildrenKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(
		childrenKey,
		n.ix.annotationKey,
		n.path,
		n.prefix,
	)
}

func (n *node) getPrefixAddressOfChild(childByte byte) common.Hash {
	return crypto.Keccak256Hash(
		prefixesKey,
		n.ix.annotationKey,
		n.path,
		n.prefix,
		[]byte{childByte},
	)
}

func (n *node) isEmpty() bool {
	numOfChildren := bytekeyset.Size(n.ix.db, n.getChildrenKeySetAddress())
	numOfEntities := keyset.Size(n.ix.db, n.getEntityKeySetAddress())
	numOfSuffixEntities := keyset.Size(n.ix.db, n.getSuffixEntityArrayAddress())

	return numOfChildren.CmpUint64(0) == 0 &&
		numOfEntities.CmpUint64(0) == 0 &&
		numOfSuffixEntities.CmpUint64(0) == 0
}

func (n *node) splitChild(remaining []byte, child *node) error {
	existingChildPrefix := child.prefix

	// Find the common prefix between the two, this will be the new prefix of the child
	prefix := findCommonPrefix(remaining, child.prefix)
	if len(prefix) == 0 {
		return fmt.Errorf("error splitting node, prefix was empty, this should never happen")
	}

	// Set the new prefix for the child
	child.prefix = prefix
	// Rewrite the prefix of the modified child
	n.setPrefixOf(child)

	// The new prefix for the existing child that we pushed down
	newChildPrefix, found := bytes.CutPrefix(existingChildPrefix, prefix)
	if !found {
		return fmt.Errorf("error splitting node, the new prefix isn't a prefix, this should never happen")
	}
	// By creating this child, it should automatically have its keysets pointing
	// to the sets that already existed before the split, since the addresses of
	// these sets are calculated purely based on the concatenation of the node's
	// path and prefix
	newChild, err := child.addChild(newChildPrefix)
	if err != nil {
		return fmt.Errorf("error splitting node: %v", err)
	}

	if newChild.isEmpty() {
		return fmt.Errorf("error splitting node, the new child resulting from the split was empty, this should never happen")
	}

	rest, found := bytes.CutPrefix(remaining, prefix)
	if !found {
		return fmt.Errorf("error splitting node, the new prefix isn't a prefix, this should never happen")
	}
	if len(rest) > 0 {
		if _, err := child.addChild([]byte(rest)); err != nil {
			return fmt.Errorf("error splitting node: %v", err)
		}
	}

	return nil
}

func (n *node) fuse() {
	// We can't fuse the root node, which is the only node with an empty prefix
	// We can only fuse a node with its child if it has only one child, and if the
	// node doesn't store any entities or suffixes
	if len(n.prefix) != 0 &&
		bytekeyset.Size(n.ix.db, n.getChildrenKeySetAddress()).CmpUint64(1) == 0 &&
		keyset.Size(n.ix.db, n.getEntityKeySetAddress()).CmpUint64(0) == 0 &&
		keyset.Size(n.ix.db, n.getSuffixEntityArrayAddress()).CmpUint64(0) == 0 {
		// We know that this will only iterate once
		for child := range n.getChildren() {
			fusedPrefix := slices.Concat(n.prefix, child.prefix)

			if len(fusedPrefix) <= 32 {
				bytekeyset.Clear(n.ix.db, n.getChildrenKeySetAddress())
				n.ix.db.SetState(storageutil.GolemDBAddress, n.getPrefixAddressOfChild(child.prefix[0]), common.Hash{})

				n.prefix = fusedPrefix
				prefixAddress := crypto.Keccak256Hash(
					prefixesKey,
					n.ix.annotationKey,
					n.path,
					n.prefix[0:1],
				)
				n.ix.db.SetState(
					storageutil.GolemDBAddress,
					prefixAddress,
					common.BytesToHash(n.prefix),
				)
			}
		}
	}
}

func (n *node) addEntity(value string, entityKeys ...common.Hash) error {
	return n.doAddEntity(&strings.Builder{}, []byte(value), entityKeys...)
}

func (n *node) doAddEntity(seen *strings.Builder, remaining []byte, entityKeys ...common.Hash) error {

	if len(remaining) == 0 {
		// We found an existing node for the annotation value, so we just add the entities to it
		for _, entityKey := range entityKeys {
			err := keyset.AddValue(n.ix.db, n.getEntityKeySetAddress(), entityKey)
			if err != nil {
				return fmt.Errorf("error adding entity to index: %v", err)
			}
		}
		return nil
	}

	// Get the child corresponding to the next byte
	child := n.getChild(remaining[0])
	if child == nil {
		// No such child exists, so let's create one
		newChild, err := n.addChild([]byte(remaining))
		if err != nil {
			return fmt.Errorf("error adding entity to index: %v", err)
		}
		child = newChild
	}

	// Check whether the child prefix is a prefix of the remaining search string
	// this includes the case where the child prefix equals the remaining search string
	if rest, found := bytes.CutPrefix(remaining, child.prefix); found {
		// The child prefix is a prefix, so we can simply consume the prefix and recurse
		if _, err := seen.Write(child.prefix); err != nil {
			return fmt.Errorf("error adding entity to index: %v", err)
		}
		return child.doAddEntity(seen, rest, entityKeys...)
	} else {
		// The child prefix is not a prefix of the remaining search string.
		// The child matches at least one byte though, so we need to split the child
		if err := n.splitChild(remaining, child); err != nil {
			return fmt.Errorf("error while adding entity to index: %v", err)
		}

		// We try to add the remaining prefix again to the splitted node
		return n.doAddEntity(seen, remaining, entityKeys...)
	}
}

func (n *node) addSuffix(fullPath string, suffix string, entityKeys ...common.Hash) error {
	return n.doAddSuffix([]byte(fullPath), &strings.Builder{}, []byte(suffix), entityKeys...)
}

func (n *node) doAddSuffix(fullPath []byte, seen *strings.Builder, remaining []byte, entityKeys ...common.Hash) error {
	if len(remaining) == 0 {
		// We found an existing node for the annotation value
		arr := array.NewArray(n.ix.db, n.getSuffixEntityArrayAddress())
		keysetAddress := n.ix.getEntityKeySetAddress(fullPath)
		found := false
		for addr := range arr.Iterate {
			if addr.Cmp(keysetAddress) == 0 {
				found = true
				break
			}
		}
		if !found {
			arr.Append(keysetAddress)
		}
		return nil
	}

	// Get the child corresponding to the next byte
	child := n.getChild(remaining[0])
	if child == nil {
		// No such child exists, so let's create one
		newChild, err := n.addChild([]byte(remaining))
		if err != nil {
			return err
		}
		child = newChild
	}

	// Check whether the child prefix is a prefix of the remaining search string
	// this includes the case where the child prefix equals the remaining search string
	if rest, found := bytes.CutPrefix(remaining, child.prefix); found {
		// The child prefix is a prefix, so we can simply consume the prefix and recurse
		if _, err := seen.Write(child.prefix); err != nil {
			return fmt.Errorf("error adding entity to index: %v", err)
		}
		return child.doAddSuffix(fullPath, seen, rest, entityKeys...)
	} else {
		// The child prefix is not a prefix of the remaining search string.
		// The child matches at least one byte though, so we need to split the child
		if err := n.splitChild(remaining, child); err != nil {
			return fmt.Errorf("error while adding entity to index: %v", err)
		}

		// We try to add the remaining suffix again to the splitted node
		return n.doAddSuffix(fullPath, seen, remaining, entityKeys...)
	}
}

func (n *node) getChild(b byte) *node {
	if bytekeyset.ContainsValue(n.ix.db, n.getChildrenKeySetAddress(), b) {
		prefixHash := n.ix.db.GetState(storageutil.GolemDBAddress, n.getPrefixAddressOfChild(b))
		// Trim the extra zero bytes that were introduced when we converted to a 32 byte hash
		prefix := bytes.TrimLeft(prefixHash.Bytes(), "\x00")

		// We don't store single-byte prefixes
		if len(prefix) == 0 {
			prefix = []byte{b}
		}
		if prefix[0] != b {
			fmt.Printf("%s -> %s\n", string(b), string(prefix))
			panic("the child prefix does not correspond to the input byte")
		}

		child := &node{
			ix:     n.ix,
			path:   slices.Concat(n.path, n.prefix),
			prefix: prefix,
		}
		return child
	}
	return nil
}

func (n *node) addChild(prefix []byte) (*node, error) {
	// Cap the prefix to maximum 32 bytes, which is what we can store in a single slot
	prefix = prefix[:min(32, len(prefix))]

	child := &node{
		ix:     n.ix,
		path:   slices.Concat(n.path, n.prefix),
		prefix: prefix,
	}

	// The key is the first byte of the child's prefix
	key := child.prefix[0]
	if bytekeyset.ContainsValue(n.ix.db, n.getChildrenKeySetAddress(), key) {
		return nil, fmt.Errorf("key %s already present in the keyset, this is a bug", string(key))
	}

	// Register the new child as a child of n
	err := bytekeyset.AddValue(n.ix.db, n.getChildrenKeySetAddress(), key)
	if err != nil {
		return nil, fmt.Errorf("error adding child to node: %v", err)
	}

	// Write the full prefix of the child to the right address
	n.setPrefixOf(child)

	return child, nil
}

func (n *node) setPrefixOf(child *node) {
	// We only write the prefix if it's longer than one byte
	if len(child.prefix) > 1 {
		n.ix.db.SetState(
			storageutil.GolemDBAddress,
			n.getPrefixAddressOfChild(child.prefix[0]),
			common.BytesToHash(child.prefix),
		)
	} else {
		n.ix.db.SetState(
			storageutil.GolemDBAddress,
			n.getPrefixAddressOfChild(child.prefix[0]),
			common.Hash{},
		)
	}
}

func (n *node) removeEntity(value string, entityKeys ...common.Hash) error {
	_, err := n.doRemoveEntity(&strings.Builder{}, []byte(value), entityKeys...)
	return err
}

// removeEntity removes the given entity from the given value, and returns
// a boolean indicating whether this removal led to the node becoming empty
func (n *node) doRemoveEntity(seen *strings.Builder, remaining []byte, entityKeys ...common.Hash) (bool, error) {

	if len(remaining) == 0 {
		for _, entityKey := range entityKeys {
			err := keyset.RemoveValue(n.ix.db, n.getEntityKeySetAddress(), entityKey)
			if err != nil {
				return false, fmt.Errorf("error removing entity from index: %v", err)
			}
		}
	} else {

		childByte := remaining[0]
		child := n.getChild(childByte)

		if child != nil {

			rest, found := bytes.CutPrefix(remaining, child.prefix)
			if found {

				if _, err := seen.Write(child.prefix); err != nil {
					return false, fmt.Errorf("error removing entity from index: %v", err)
				}

				erased, err := child.doRemoveEntity(seen, rest, entityKeys...)
				if err != nil {
					return false, fmt.Errorf("error removing entity from index: %v", err)
				}
				if erased {
					err := bytekeyset.RemoveValue(n.ix.db, n.getChildrenKeySetAddress(), childByte)
					if err != nil {
						return false, fmt.Errorf("error removing entity from index: %v", err)
					}

					n.ix.db.SetState(storageutil.GolemDBAddress, n.getPrefixAddressOfChild(childByte), common.Hash{})

					// Try to fuse the node to compact the trie
					n.fuse()
				}
			}
		}
	}

	return n.isEmpty(), nil
}

func (n *node) removeSuffix(fullPath string, value string, entityKeys ...common.Hash) error {
	_, err := n.doRemoveSuffix([]byte(fullPath), &strings.Builder{}, []byte(value), entityKeys...)
	return err
}

func (n *node) doRemoveSuffix(fullPath []byte, seen *strings.Builder, remaining []byte, entityKeys ...common.Hash) (bool, error) {

	if len(remaining) == 0 {
		arr := array.NewArray(n.ix.db, n.getSuffixEntityArrayAddress())
		targetAddress := n.ix.getEntityKeySetAddress(fullPath)
		// We shouldn't modify the array while iterating over it, so just find the index,
		// and then remove the value
		indexToRemove := uint256.NewInt(0)
		for ix, address := range arr.IterateWithIndex {
			if address.Cmp(targetAddress) == 0 {
				indexToRemove = ix
				break
			}
		}
		arr.RemoveUnordered(indexToRemove)

		for _, entityKey := range entityKeys {
			err := keyset.RemoveValue(n.ix.db, n.getSuffixEntityArrayAddress(), entityKey)
			if err != nil {
				return false, fmt.Errorf("error removing suffix from index: %v", err)
			}
		}
	} else {

		childByte := remaining[0]
		child := n.getChild(childByte)

		if child != nil {

			rest, found := bytes.CutPrefix(remaining, child.prefix)
			if found {

				if _, err := seen.Write(child.prefix); err != nil {
					return false, fmt.Errorf("error removing suffix from index: %v", err)
				}

				erased, err := child.doRemoveSuffix(fullPath, seen, rest, entityKeys...)
				if err != nil {
					return false, fmt.Errorf("error removing suffix from index: %v", err)
				}
				if erased {
					err := bytekeyset.RemoveValue(n.ix.db, n.getChildrenKeySetAddress(), childByte)
					if err != nil {
						return false, fmt.Errorf("error removing suffix from index: %v", err)
					}

					n.ix.db.SetState(storageutil.GolemDBAddress, n.getPrefixAddressOfChild(childByte), common.Hash{})

					// Try to fuse the node to compact the trie
					n.fuse()
				}
			}
		}
	}

	return n.isEmpty(), nil
}

func (n *node) getChildren() iter.Seq[*node] {
	return iter.Seq[*node](func(yield func(*node) bool) {
		bytekeyset.Iterate(n.ix.db, n.getChildrenKeySetAddress())(
			func(b byte) bool {
				return yield(n.getChild(b))
			})
	})
}

type Index struct {
	db            storageutil.StateAccess
	annotationKey []byte
}

func NewIndex(db storageutil.StateAccess, annotationKey string) *Index {
	return &Index{
		db:            db,
		annotationKey: []byte(annotationKey),
	}
}

func (ix *Index) getRootNode() *node {
	return &node{
		ix:     ix,
		path:   []byte{},
		prefix: []byte{},
	}
}

func (ix *Index) AddEntity(value string, entityKeys ...common.Hash) error {
	normalised := norm.NFC.String(value)
	rest := normalised

	isSuffix := false

	for len(rest) != 0 {
		var err error
		if !isSuffix {
			err = ix.getRootNode().addEntity(rest, entityKeys...)
		} else {
			err = ix.getRootNode().addSuffix(normalised, rest, entityKeys...)
		}
		if err != nil {
			return fmt.Errorf("error adding entity to index: %v", err)
		}

		rest = rest[1:]
		isSuffix = true
	}
	return nil
}

func (ix *Index) RemoveEntity(value string, entityKey ...common.Hash) error {
	normalised := norm.NFC.String(value)
	rest := normalised

	isSuffix := false
	for len(rest) != 0 {
		var err error
		if !isSuffix {
			err = ix.getRootNode().removeEntity(rest, entityKey...)
		} else {
			err = ix.getRootNode().removeSuffix(normalised, rest, entityKey...)
		}
		if err != nil {
			return fmt.Errorf("error removing entity from index: %v", err)
		}

		rest = rest[1:]
		isSuffix = true
	}
	return nil
}

func (ix *Index) FindEntitiesStartingWith(pattern string) iter.Seq[common.Hash] {

	rest := []byte(norm.NFC.String(pattern))

	n := ix.getRootNode()
	for len(rest) != 0 {
		n = n.getChild(rest[0])

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		var found = false
		prefix := findCommonPrefix(rest, n.prefix)
		if len(prefix) == 0 {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest, found = bytes.CutPrefix(rest, prefix)
		if !found {
			panic("this should not happen")
		}
	}

	var doIterate func(n *node, yield func(common.Hash) bool) bool
	doIterate = func(n *node, yield func(common.Hash) bool) bool {

		finished := false
		yield_ := func(hash common.Hash) bool {
			finished = !yield(hash)
			return !finished
		}

		keyset.Iterate(ix.db, n.getEntityKeySetAddress())(yield_)
		if finished {
			return !finished
		}

		for child := range n.getChildren() {
			if !finished {
				finished = !doIterate(child, yield)
			}
		}
		return !finished
	}

	return iter.Seq[common.Hash](func(yield func(common.Hash) bool) {
		seen := make(map[common.Hash]struct{})
		doIterate(n, func(hash common.Hash) bool {
			_, found := seen[hash]
			if !found {
				seen[hash] = struct{}{}
				return yield(hash)
			}
			return true
		})
	})
}

func (ix *Index) FindEntitiesEndingWith(pattern string) iter.Seq[common.Hash] {

	rest := []byte(norm.NFC.String(pattern))

	n := ix.getRootNode()
	for len(rest) != 0 {
		n = n.getChild(rest[0])

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		var found = false
		prefix := findCommonPrefix(rest, n.prefix)
		if len(prefix) == 0 || len(n.prefix) > len(rest) {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest, found = bytes.CutPrefix(rest, prefix)
		if !found {
			panic("this should not happen")
		}
	}

	return iter.Seq[common.Hash](func(yield func(common.Hash) bool) {
		seen := make(map[common.Hash]struct{})
		finished := false
		yield_ := func(hash common.Hash) bool {
			_, found := seen[hash]
			if !found {
				seen[hash] = struct{}{}
				finished = !yield(hash)
				return !finished
			}
			return true
		}

		keyset.Iterate(ix.db, n.getEntityKeySetAddress())(yield_)
		if !finished {
			a := array.NewArray(ix.db, n.getSuffixEntityArrayAddress())
			for keysetAddress := range a.Iterate {
				keyset.Iterate(ix.db, keysetAddress)(yield_)
				if finished {
					return
				}
			}
		}
	})
}

func (ix *Index) FindEntitiesContaining(pattern string) iter.Seq[common.Hash] {

	rest := []byte(norm.NFC.String(pattern))

	n := ix.getRootNode()
	for len(rest) != 0 {
		n = n.getChild(rest[0])

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		var found = false
		prefix := findCommonPrefix(rest, n.prefix)
		if len(prefix) == 0 {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest, found = bytes.CutPrefix(rest, prefix)
		if !found {
			panic("this should not happen")
		}
	}

	var doIterate func(n *node, yield func(common.Hash) bool) bool
	doIterate = func(n *node, yield func(common.Hash) bool) bool {

		finished := false
		yield_ := func(hash common.Hash) bool {
			finished = !yield(hash)
			return !finished
		}

		keyset.Iterate(ix.db, n.getEntityKeySetAddress())(yield_)
		if finished {
			return !finished
		}

		a := array.NewArray(ix.db, n.getSuffixEntityArrayAddress())
		for keysetAddress := range a.Iterate {
			keyset.Iterate(ix.db, keysetAddress)(yield_)
			if finished {
				return !finished
			}
		}

		for child := range n.getChildren() {
			if !finished {
				finished = !doIterate(child, yield)
			}
		}
		return !finished
	}

	return iter.Seq[common.Hash](func(yield func(common.Hash) bool) {
		seen := make(map[common.Hash]struct{})
		doIterate(n, func(hash common.Hash) bool {
			_, found := seen[hash]
			if !found {
				seen[hash] = struct{}{}
				return yield(hash)
			}
			return true
		})
	})
}
