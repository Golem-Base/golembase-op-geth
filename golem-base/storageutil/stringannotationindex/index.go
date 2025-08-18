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

type EntryType uint

const (
	Undefined EntryType = iota
	RealEntry
	SuffixEntry
)

func findCommonPrefix(s1 []byte, s2 []byte) []byte {
	prefix := bytes.Buffer{}
	for i := 0; i < min(len(s1), len(s2)) && s1[i] == s2[i]; i++ {
		prefix.WriteByte(s1[i])
	}
	return prefix.Bytes()
}

func (n *node) getEntityKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(
		entitiesSetKey,
		n.ix.annotationKey,
		n.path,
		n.prefix,
	)
}

func (n *node) getSuffixEntityKeySetAddress() common.Hash {
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
	numOfSuffixEntities := keyset.Size(n.ix.db, n.getSuffixEntityKeySetAddress())

	return numOfChildren.CmpUint64(0) == 0 &&
		numOfEntities.CmpUint64(0) == 0 &&
		numOfSuffixEntities.CmpUint64(0) == 0
}

func (n *node) AddEntity(value string, entryType EntryType, entityKeys ...common.Hash) error {
	return n.addEntity(&strings.Builder{}, []byte(value), entryType, entityKeys...)
}

func (n *node) getEntityKeySetAddressByType(entryType EntryType) (common.Hash, error) {
	switch entryType {
	case RealEntry:
		return n.getEntityKeySetAddress(), nil
	case SuffixEntry:
		return n.getSuffixEntityKeySetAddress(), nil
	default:
		return common.Hash{}, fmt.Errorf("tried to add an entity with an undefined entry type")
	}
}

func (n *node) addEntity(
	seen *strings.Builder,
	remaining []byte,
	entryType EntryType,
	entityKeys ...common.Hash) error {

	if len(remaining) == 0 {
		// We found an existing node for the annotation value, so we just add the entities to it
		keysetAddr, err := n.getEntityKeySetAddressByType(entryType)
		if err != nil {
			return fmt.Errorf("error adding entity to index: %w", err)
		}
		for _, entityKey := range entityKeys {
			err := keyset.AddValue(n.ix.db, keysetAddr, entityKey)
			if err != nil {
				return fmt.Errorf("error adding entity to index: %w", err)
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
			return err
		}
		child = newChild
	}

	// Check whether the child prefix is a prefix of the remaining search string
	// this includes the case where the child prefix equals the remaining search string
	if rest, found := bytes.CutPrefix(remaining, child.prefix); found {
		// The child prefix is a prefix, so we can simply consume the prefix and recurse
		if _, err := seen.Write(child.prefix); err != nil {
			return fmt.Errorf("error adding entity to index: %w", err)
		}
		return child.addEntity(seen, rest, entryType, entityKeys...)
	} else {
		// The child prefix is not a prefix of the remaining search string.
		// The child matches at least one byte though, so need to split the child

		existingChildPrefix := child.prefix

		// Find the common prefix between the two, this will be the new prefix of the child
		prefix := findCommonPrefix(remaining, child.prefix)
		if len(prefix) == 0 {
			panic("prefix was empty, this should never happen")
		}

		// Set the new prefix for the child
		child.prefix = prefix
		// Rewrite the prefix of the modified child
		n.setPrefixOf(child)

		// The new prefix for the existing child that we pushed down
		newChildPrefix, found := bytes.CutPrefix(existingChildPrefix, prefix)
		if !found {
			panic("this should never happen")
		}
		// By creating this child, it should automatically have its keysets pointing
		// to the sets that already existed before the split, since the addresses of
		// these sets are calculated purely based on the concatenation of the node's
		// path and prefix
		newChild, err := child.addChild(newChildPrefix)
		if err != nil {
			return fmt.Errorf("error splitting node: %w", err)
		}

		if newChild.isEmpty() {
			panic("the new child resulting from the split was empty, this should never happen")
		}

		rest, found := bytes.CutPrefix(remaining, prefix)
		if !found {
			panic("this should never happen")
		}
		if len(rest) > 0 {
			if _, err := child.addChild([]byte(rest)); err != nil {
				return fmt.Errorf("error splitting node: %w", err)
			}
		}

		return n.addEntity(seen, remaining, entryType, entityKeys...)
	}
}

func (n *node) getChild(b byte) *node {
	if bytekeyset.ContainsValue(n.ix.db, n.getChildrenKeySetAddress(), b) {
		prefixHash := n.ix.db.GetState(storageutil.GolemDBAddress, n.getPrefixAddressOfChild(b))
		// Trim the extra zero bytes that were introduced when we converted to a 32 byte hash
		prefix := bytes.TrimLeft(prefixHash.Bytes(), "\x00")

		if len(prefix) == 0 {
			panic("empty prefix, this should not happen")
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
		return nil, fmt.Errorf("key already present in the keyset, this is a bug")
	}

	// Register the new child as a child of n
	err := bytekeyset.AddValue(n.ix.db, n.getChildrenKeySetAddress(), key)
	if err != nil {
		return nil, fmt.Errorf("error adding child to node: %w", err)
	}

	// Write the full prefix of the child to the right address
	n.setPrefixOf(child)

	return child, nil
}

func (n *node) setPrefixOf(child *node) {
	n.ix.db.SetState(
		storageutil.GolemDBAddress,
		n.getPrefixAddressOfChild(child.prefix[0]),
		common.BytesToHash(child.prefix),
	)
}

func (n *node) RemoveEntity(value string, entryType EntryType, entityKeys ...common.Hash) (bool, error) {
	return n.removeEntity(&strings.Builder{}, []byte(value), entryType, entityKeys...)
}

// removeEntity removes the given entity from the given value, and returns
// a boolean indicating whether this removal led to the node becoming empty
func (n *node) removeEntity(
	seen *strings.Builder,
	remaining []byte,
	entryType EntryType,
	entityKeys ...common.Hash) (bool, error) {

	if len(remaining) == 0 {
		keysetAddr, err := n.getEntityKeySetAddressByType(entryType)
		if err != nil {
			return false, fmt.Errorf("error removing entity from index: %w", err)
		}
		for _, entityKey := range entityKeys {
			if !keyset.ContainsValue(n.ix.db, keysetAddr, entityKey) {
				return false, fmt.Errorf("keyset does not contain entity")
			}
			err := keyset.RemoveValue(n.ix.db, keysetAddr, entityKey)
			if err != nil {
				return false, fmt.Errorf("error removing entity from index: %w", err)
			}
		}
	} else {

		childByte := remaining[0]
		child := n.getChild(childByte)

		if child != nil {

			rest, found := bytes.CutPrefix(remaining, child.prefix)
			if found {

				if _, err := seen.Write(child.prefix); err != nil {
					return false, fmt.Errorf("error removing entity from index: %w", err)
				}

				erased, err := child.removeEntity(seen, rest, entryType, entityKeys...)
				if err != nil {
					return false, fmt.Errorf("error removing entity from index: %w", err)
				}
				if erased {
					if !bytekeyset.ContainsValue(n.ix.db, n.getChildrenKeySetAddress(), childByte) {
						return false, fmt.Errorf("keyset does not contain child")
					}
					err := bytekeyset.RemoveValue(n.ix.db, n.getChildrenKeySetAddress(), childByte)
					if err != nil {
						return false, fmt.Errorf("error removing entity from index: %w", err)
					}

					n.ix.db.SetState(storageutil.GolemDBAddress, n.getPrefixAddressOfChild(childByte), common.Hash{})

					// If possible, fuse nodes together again
					if len(n.prefix) != 0 &&
						bytekeyset.Size(n.ix.db, n.getChildrenKeySetAddress()).CmpUint64(1) == 0 &&
						keyset.Size(n.ix.db, n.getEntityKeySetAddress()).CmpUint64(0) == 0 &&
						keyset.Size(n.ix.db, n.getSuffixEntityKeySetAddress()).CmpUint64(0) == 0 {
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
									slices.Concat(n.path, n.prefix[0:1]),
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

func (ix *Index) addEntity(value string, entityKeys ...common.Hash) error {
	rest := norm.NFC.String(value)
	entryType := RealEntry
	for len(rest) != 0 {
		err := ix.getRootNode().AddEntity(rest, entryType, entityKeys...)
		if err != nil {
			return fmt.Errorf("error adding entity to index: %w", err)
		}

		rest = rest[1:]
		entryType = SuffixEntry
	}
	return nil
}

func (ix *Index) removeEntity(value string, entityKey common.Hash) error {
	rest := norm.NFC.String(value)
	entryType := RealEntry
	for len(rest) != 0 {
		_, err := ix.getRootNode().RemoveEntity(rest, entryType, entityKey)
		if err != nil {
			return fmt.Errorf("error removing entity from index: %w", err)
		}

		rest = rest[1:]
		entryType = SuffixEntry
	}
	return nil
}

func (ix *Index) findEntitiesStartingWith(pattern string) iter.Seq[common.Hash] {

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

func (ix *Index) findEntitiesEndingWith(pattern string) iter.Seq[common.Hash] {

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
			keyset.Iterate(ix.db, n.getSuffixEntityKeySetAddress())(yield_)
		}
	})
}

func (ix *Index) findEntitiesContaining(pattern string) iter.Seq[common.Hash] {

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

		keyset.Iterate(ix.db, n.getSuffixEntityKeySetAddress())(yield_)
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
