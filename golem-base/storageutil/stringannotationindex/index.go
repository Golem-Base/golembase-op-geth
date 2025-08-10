// Package stringannotationindex defines a data structure to efficiently query string annotations
package stringannotationindex

import (
	"fmt"
	"iter"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/holiman/uint256"
	"golang.org/x/text/unicode/norm"
)

var entitiesSetPrefix = []byte("golemBase.stringannotationindex.entities")
var suffixEntitiesSetPrefix = []byte("golemBase.stringannotationindex.entities.suffix")
var charactersSetPrefix = []byte("golemBase.stringannotationindex.characters")

type node struct {
	ix      *Index
	address common.Hash
}

func addToAddress(address common.Hash, offset uint64) []byte {
	addrInt := new(uint256.Int).SetBytes(address.Bytes())
	addrInt.AddUint64(addrInt, offset)
	return addrInt.Bytes()
}

func (n *node) getEntityKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(entitiesSetPrefix, addToAddress(n.address, 1))
}

func (n *node) getSuffixEntityKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(suffixEntitiesSetPrefix, addToAddress(n.address, 1))
}

func (n *node) getCharactersKeySetAddress() common.Hash {
	return crypto.Keccak256Hash(charactersSetPrefix, addToAddress(n.address, 1))
}

func (n *node) isEmpty() bool {
	numOfChildren := keyset.Size(n.ix.db, n.getCharactersKeySetAddress())
	numOfEntities := keyset.Size(n.ix.db, n.getEntityKeySetAddress())
	numOfSuffixEntities := keyset.Size(n.ix.db, n.getSuffixEntityKeySetAddress())

	return numOfChildren.CmpUint64(0) == 0 &&
		numOfEntities.CmpUint64(0) == 0 &&
		numOfSuffixEntities.CmpUint64(0) == 0
}

func (n *node) addEntity(value string, isSuffix bool, entityKeys ...common.Hash) error {
	if len(value) == 0 {
		for _, entityKey := range entityKeys {
			var err error
			if isSuffix {
				err = keyset.AddValue(n.ix.db, n.getSuffixEntityKeySetAddress(), entityKey)
			} else {
				err = keyset.AddValue(n.ix.db, n.getEntityKeySetAddress(), entityKey)
			}
			if err != nil {
				return fmt.Errorf("error adding entity to index: %w", err)
			}
		}
		return nil
	}

	rune, length := utf8.DecodeRuneInString(value)
	rest := value[length:]

	child := n.getChild(rune)
	if child == nil {
		newChild, err := n.addChild(rune)
		if err != nil {
			return err
		}
		child = newChild
	}

	child.addEntity(rest, isSuffix, entityKeys...)

	return nil
}

// removeEntity removes the given entity from the given value, and returns
// a boolean indicating whether this removal led to the node becoming empty
func (n *node) removeEntity(value string, isSuffix bool, entityKey common.Hash) (bool, error) {
	if len(value) == 0 {
		var err error
		if isSuffix {
			err = keyset.RemoveValue(n.ix.db, n.getSuffixEntityKeySetAddress(), entityKey)
		} else {
			err = keyset.RemoveValue(n.ix.db, n.getEntityKeySetAddress(), entityKey)
		}
		if err != nil {
			return false, fmt.Errorf("error removing entity from index: %w", err)
		}
	} else {
		rune, length := utf8.DecodeRuneInString(value)
		rest := value[length:]

		child := n.getChild(rune)
		if child != nil {
			erased, err := child.removeEntity(rest, isSuffix, entityKey)
			if err != nil {
				return false, fmt.Errorf("error removing entity from index: %w", err)
			}
			if erased {
				runeBytes := make([]byte, 32)
				utf8.EncodeRune(runeBytes, rune)
				keyset.RemoveValue(n.ix.db, n.getCharactersKeySetAddress(), common.BytesToHash(runeBytes))
			}
		}
	}

	return n.isEmpty(), nil
}

func (n *node) getChild(rune rune) *node {
	runeBytes := make([]byte, 32)
	utf8.EncodeRune(runeBytes, rune)
	childAddress := crypto.Keccak256Hash(n.address.Bytes(), runeBytes)

	if keyset.ContainsValue(n.ix.db, n.getCharactersKeySetAddress(), common.BytesToHash(runeBytes)) {
		return &node{
			ix:      n.ix,
			address: childAddress,
		}
	}

	return nil
}

func (n *node) addChild(rune rune) (*node, error) {
	runeBytes := make([]byte, 32)
	utf8.EncodeRune(runeBytes, rune)
	childAddress := crypto.Keccak256Hash(n.address.Bytes(), runeBytes)

	child := &node{
		ix:      n.ix,
		address: childAddress,
	}

	err := keyset.AddValue(n.ix.db, n.getCharactersKeySetAddress(), common.BytesToHash(runeBytes))
	if err != nil {
		return nil, fmt.Errorf("error adding child to node: %w", err)
	}

	return child, nil
}

func (n *node) getChildren() iter.Seq[*node] {
	return iter.Seq[*node](func(yield func(*node) bool) {
		keyset.Iterate(n.ix.db, n.getCharactersKeySetAddress())(
			func(runeHash common.Hash) bool {
				childAddress := crypto.Keccak256Hash(n.address.Bytes(), runeHash.Bytes())
				child := n.ix.getNodeAt(childAddress)
				return yield(child)
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

func (ix *Index) getNodeAt(address common.Hash) *node {
	return &node{
		ix:      ix,
		address: address,
	}
}

func (ix *Index) getRootHash() common.Hash {
	return annotationindex.StringAnnotationIndexRootHash(ix.annotationKey)
}

func (ix *Index) getRootNode() *node {
	return ix.getNodeAt(ix.getRootHash())
}

func (ix *Index) addEntity(value string, entityKeys ...common.Hash) error {
	rest := norm.NFC.String(value)
	isSuffix := false
	for len(rest) != 0 {
		err := ix.getRootNode().addEntity(rest, isSuffix, entityKeys...)
		if err != nil {
			return fmt.Errorf("error adding entity to index: %w", err)
		}

		_, length := utf8.DecodeRuneInString(rest)
		rest = rest[length:]
		isSuffix = true
	}
	return nil
}

func (ix *Index) removeEntity(value string, entityKey common.Hash) error {
	rest := norm.NFC.String(value)
	isSuffix := false
	for len(rest) != 0 {
		_, err := ix.getRootNode().removeEntity(rest, isSuffix, entityKey)
		if err != nil {
			return fmt.Errorf("error removing entity to index: %w", err)
		}

		_, length := utf8.DecodeRuneInString(rest)
		rest = rest[length:]
		isSuffix = true
	}
	return nil
}

func (ix *Index) findEntitiesStartingWith(pattern string) iter.Seq[common.Hash] {

	rest := norm.NFC.String(pattern)

	n := ix.getRootNode()
	for len(rest) != 0 {
		rune, length := utf8.DecodeRuneInString(rest)
		n = n.getChild(rune)

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest = rest[length:]
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
		rune, length := utf8.DecodeRuneInString(rest)
		n = n.getChild(rune)

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest = rest[length:]
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
		rune, length := utf8.DecodeRuneInString(rest)
		n = n.getChild(rune)

		if n == nil {
			// No matches
			return func(func(common.Hash) bool) {}
		}

		rest = rest[length:]
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
