// Package stringannotationindex defines a data structure to efficiently query string annotations
package stringannotationindex

import (
	"bytes"
	"fmt"
	"iter"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"golang.org/x/text/unicode/norm"
)

var entitiesSetPrefix = []byte("golemBase.stringannotationindex.entities")
var suffixEntitiesSetPrefix = []byte("golemBase.stringannotationindex.entities.suffix")
var childrenSetPrefix = []byte("golemBase.stringannotationindex.characters")
var fullPathPrefix = []byte("golemBase.stringannotationindex.fullpath")

type node struct {
	ix   *Index
	path []byte
}

type EntryType uint

const (
	Undefined EntryType = iota
	RealEntry
	SuffixEntry
)

func (n *node) getEntityKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(entitiesSetPrefix, []byte(n.ix.annotationKey), n.path)
}

func (n *node) getSuffixEntityKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(suffixEntitiesSetPrefix, []byte(n.ix.annotationKey), n.path)
}

func (n *node) getChildrenKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(childrenSetPrefix, []byte(n.ix.annotationKey), n.path)
}

func (n *node) getFullPathAddressOfChild(childByte byte) common.Hash {
	path := slices.Concat(n.path, []byte{childByte})
	return crypto.Keccak256Hash(fullPathPrefix, []byte(n.ix.annotationKey), path)
}

func (n *node) isEmpty() bool {
	numOfChildren := keyset.Size(n.ix.db, n.getChildrenKeySetAddress())
	numOfEntities := keyset.Size(n.ix.db, n.getEntityKeySetAddress())
	numOfSuffixEntities := keyset.Size(n.ix.db, n.getSuffixEntityKeySetAddress())

	return numOfChildren.CmpUint64(0) == 0 &&
		numOfEntities.CmpUint64(0) == 0 &&
		numOfSuffixEntities.CmpUint64(0) == 0
}

func (n *node) AddEntity(value string, entryType EntryType, entityKeys ...common.Hash) error {
	return n.addEntity(&strings.Builder{}, value, entryType, entityKeys...)
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
	remaining string,
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

	seen.WriteByte(remaining[0])
	rest := remaining[1:]
	childBytes := []byte{remaining[0]}

	child := n.getChild(childBytes[0])
	if child == nil {
		newChild, err := n.addChild(childBytes)
		if err != nil {
			return err
		}
		child = newChild
	}

	child.addEntity(seen, rest, entryType, entityKeys...)

	return nil
}

func (n *node) RemoveEntity(value string, entryType EntryType, entityKeys ...common.Hash) (bool, error) {
	return n.removeEntity(&strings.Builder{}, value, entryType, entityKeys...)
}

// removeEntity removes the given entity from the given value, and returns
// a boolean indicating whether this removal led to the node becoming empty
func (n *node) removeEntity(
	seen *strings.Builder,
	remaining string,
	entryType EntryType,
	entityKeys ...common.Hash) (bool, error) {
	if len(remaining) == 0 {
		keysetAddr, err := n.getEntityKeySetAddressByType(entryType)
		if err != nil {
			return false, fmt.Errorf("error removing entity from index: %w", err)
		}
		for _, entityKey := range entityKeys {
			err := keyset.RemoveValue(n.ix.db, keysetAddr, entityKey)
			if err != nil {
				return false, fmt.Errorf("error removing entity from index: %w", err)
			}
		}
	} else {

		seen.WriteByte(remaining[0])
		rest := remaining[1:]
		childByte := remaining[0]

		child := n.getChild(childByte)
		if child != nil {
			erased, err := child.removeEntity(seen, rest, entryType, entityKeys...)
			if err != nil {
				return false, fmt.Errorf("error removing entity from index: %w", err)
			}
			if erased {
				err := keyset.RemoveValue(n.ix.db, n.getChildrenKeySetAddress(), common.BytesToHash([]byte{childByte}))
				if err != nil {
					return false, fmt.Errorf("error removing entity from index: %w", err)
				}
				n.ix.db.SetState(storageutil.GolemDBAddress, n.getFullPathAddressOfChild(childByte), common.Hash{})
			}
		}
	}

	return n.isEmpty(), nil
}

func (n *node) getChild(b byte) *node {
	if keyset.ContainsValue(n.ix.db, n.getChildrenKeySetAddress(), common.BytesToHash([]byte{b})) {
		pathHash := n.ix.db.GetState(storageutil.GolemDBAddress, n.getFullPathAddressOfChild(b))
		path := bytes.TrimLeft(pathHash.Bytes(), "\x00")

		return &node{
			ix:   n.ix,
			path: path,
		}
	}
	return nil
}

func (n *node) addChild(bs []byte) (*node, error) {
	path := slices.Concat(n.path, bs)

	child := &node{
		ix:   n.ix,
		path: path,
	}

	err := keyset.AddValue(n.ix.db, n.getChildrenKeySetAddress(), common.BytesToHash([]byte{bs[0]}))
	if err != nil {
		return nil, fmt.Errorf("error adding child to node: %w", err)
	}

	// The returned hash is the previous value at the address that we wrote to
	n.ix.db.SetState(storageutil.GolemDBAddress, n.getFullPathAddressOfChild(bs[0]), common.BytesToHash(path))

	return child, nil
}

func (n *node) getChildren() iter.Seq[*node] {
	return iter.Seq[*node](func(yield func(*node) bool) {
		keyset.Iterate(n.ix.db, n.getChildrenKeySetAddress())(
			func(hash common.Hash) bool {
				return yield(n.getChild(bytes.TrimLeft(hash.Bytes(), "\x00")[0]))
			})
	})
}

type Index struct {
	db            storageutil.StateAccess
	annotationKey string
}

func NewIndex(db storageutil.StateAccess, annannotationKey string) *Index {
	return &Index{
		db:            db,
		annotationKey: annannotationKey,
	}
}

func (ix *Index) getRootNode() *node {
	return &node{
		ix:   ix,
		path: []byte{},
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

		_, length := utf8.DecodeRuneInString(rest)
		rest = rest[length:]
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

		_, length := utf8.DecodeRuneInString(rest)
		rest = rest[length:]
		entryType = SuffixEntry
	}
	return nil
}

func (ix *Index) findEntitiesStartingWith(pattern string) iter.Seq[common.Hash] {

	rest := norm.NFC.String(pattern)

	n := ix.getRootNode()
	for len(rest) != 0 {
		n = n.getChild(rest[0])

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest = rest[1:]
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

	rest := norm.NFC.String(pattern)

	n := ix.getRootNode()
	for len(rest) != 0 {
		n = n.getChild(rest[0])

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest = rest[1:]
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

	rest := norm.NFC.String(pattern)

	n := ix.getRootNode()
	for len(rest) != 0 {
		n = n.getChild(rest[0])

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest = rest[1:]
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
