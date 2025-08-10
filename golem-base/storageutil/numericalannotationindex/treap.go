// Package numericalannotationindex contains the data structure to efficiently query numerical annotations
package numericalannotationindex

import (
	"cmp"
	"fmt"
	"iter"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/holiman/uint256"
)

var zeroHash = common.Hash{}

// Node represents a node in the treap.
//
// A node is laid out in slots as follows:
//
// The node is written to an address using the writeTo function. The node is not
// aware of its own address. In general the address of the node will be decided
// by the node's parent, which has addresses for its left and right children.
// The root node has a fixed address.
//
// At the node's address, we only store a single 256 bit value, which consists of:
// <63 zeroes> 1 <64 zeroes> <64 bit priority> <64 bit value>
//
// The rest of the information of the node (children, and the entities it stores),
// is written to different addresses that are derived from the index' key and the node's value.
// This is done so that nodes can be relocated to a different address with a single write,
// since nodes often need to be moved to different addresses when nodes are
// added to and deleted from the tree.
//
// If we call that address x, then we store the following:
//
//	slot with address x   : left subtree, so this will hold the priority and value of the node at the left
//	slot with address x+1 : right subtree, so this will hold the priority and value of the node at the right
//	slot with address x+2 : base address for the key set holding the entities at this node, so this address is what we use to construct the key set with
//
// So, a tree with a root node R with two children, A and B, would look like this:
//
//	slot <root hash>: <63 zeroes> 1 <64 zeroes> <R priority> <R value>
//	...
//	slot <valuetoAddr(R value)>   : <63 zeroes> 1 <64 zeroes> <A priority> <A value>
//	slot <valuetoAddr(R value)+1> : <63 zeroes> 1 <64 zeroes> <B priority> <B value>
//	slot <valuetoAddr(R value)+2> : <root node entity key set>
//	...
//	slot <valuetoAddr(A value)>   : <256 zeroes>
//	slot <valuetoAddr(A value)+1> : <256 zeroes>
//	slot <valuetoAddr(A value)+2> : <node A entity key set>
//	...
//	slot <valuetoAddr(B value)>   : <256 zeroes>
//	slot <valuetoAddr(B value)+1> : <256 zeroes>
//	slot <valuetoAddr(B value)+2> : <node B entity key set>
//
// The content of the keysets is then saved in slots at addresses derived from
// the address that we constructed the key set with.
type Node struct {
	ix *annotationIndex

	annotationValue uint64
	priority        uint64

	// The number of entities stored in the tree with this node as the root
	treeSize uint64

	// subtrees
	left  common.Hash
	right common.Hash

	// Key of the keyset holding the entities for this node
	entitySetKey common.Hash
}

// Add an entity to this node.
func (n *Node) addEntity(entityKey common.Hash) error {
	beforeSize := n.numberOfEntities()
	err := keyset.AddValue(n.ix.db, n.entitySetKey, entityKey)
	if err != nil {
		return fmt.Errorf("failed to add entity to the index keyset: %w", err)
	}
	afterSize := n.numberOfEntities()
	n.treeSize += (afterSize - beforeSize)
	return nil
}

// Remove an entity from this node.
// If the node does not contain the entity, we do nothing.
func (n *Node) removeEntity(entityKey common.Hash) error {
	beforeSize := n.numberOfEntities()
	err := keyset.RemoveValue(n.ix.db, n.entitySetKey, entityKey)
	if err != nil {
		return fmt.Errorf("error removing entity from node: %w", err)
	}
	afterSize := n.numberOfEntities()
	n.treeSize -= (beforeSize - afterSize)
	return nil
}

// We only store the priority and the value, the entities and children are stored implicitly
// when the node is created. See the comment above on how nodes are stored.
func (n *Node) writeTo(address common.Hash) {
	// We pack the values as <64 zeroes> <tree size> <priority> <annotation value>
	// The size is always at least 1, guaranteeing that for an existing node, this
	// packed value will never be the zero hash
	packedValue := new(uint256.Int).SetUint64(n.treeSize)
	packedValue.Lsh(packedValue, 64)
	packedValue.AddUint64(packedValue, n.priority)
	packedValue.Lsh(packedValue, 64)
	packedValue.AddUint64(packedValue, n.annotationValue)

	n.ix.db.SetState(storageutil.GolemDBAddress, address, common.Hash(packedValue.Bytes32()))
}

func (n *Node) rotateLeft() (*Node, error) {
	oldRoot := n
	ix := oldRoot.ix

	// The node on the right of the old root, will become the new root
	newRoot := ix.getNodeAt(oldRoot.right)
	if newRoot == nil {
		return nil, fmt.Errorf("rotating to the left isn't possible if the node to the right is nil")
	}
	// The old root goes to the left
	newLeft := oldRoot
	// The node at the left of the new root, will go to the right of the old root,
	// so to the right and then to the left starting from the new root
	newLeftRight := ix.getNodeAt(newRoot.left)

	newLeft.setRight(newLeftRight)
	newRoot.setLeft(newLeft)
	return newRoot, nil
}

func (n *Node) rotateRight() (*Node, error) {
	oldRoot := n
	ix := oldRoot.ix

	// The node on the left of the old root, will become the new root
	newRoot := ix.getNodeAt(oldRoot.left)
	if newRoot == nil {
		return nil, fmt.Errorf("rotating to the right isn't possible if the node to the left is nil")
	}
	// The old root goes to the right
	newRight := oldRoot
	// The node at the right of the new root, will go to the left of the old root,
	// so to the right and then to the left starting from the new root
	newRightLeft := ix.getNodeAt(newRoot.right)

	newRight.setLeft(newRightLeft)
	newRoot.setRight(newRight)
	return newRoot, nil
}

// TreeSize returns the number of entities stored in the tree that has this node as its root.
func (n *Node) TreeSize() uint64 {
	return n.treeSize
}

// Return the number of entities stored directly in this node.
func (n *Node) numberOfEntities() uint64 {
	return keyset.Size(n.ix.db, n.entitySetKey).Uint64()
}

// Recalculate the size of the current node, given its left and right children,
// and return the updated node.
// We take the children as arguments as an optimisation to avoid needing to refetch
// them from the state.
func (n *Node) recalculateSize(left *Node, right *Node) *Node {
	newSize := n.numberOfEntities()
	if left != nil {
		newSize += left.TreeSize()
	}
	if right != nil {
		newSize += right.TreeSize()
	}

	n.treeSize = newSize
	return n
}

// Set the left child of this node to the given node, and recalculate the node's size.
func (n *Node) setLeft(left *Node) *Node {
	n.ix.writeNodeTo(left, n.left)
	return n.recalculateSize(left, n.ix.getNodeAt(n.right))
}

// Set the right child of this node to the given node, and recalculate the node's size.
func (n *Node) setRight(right *Node) *Node {
	n.ix.writeNodeTo(right, n.right)
	return n.recalculateSize(n.ix.getNodeAt(n.left), right)
}

func (n *Node) print(level int) {
	spaces := strings.Repeat(" ", level)

	fmt.Printf("%s Node at level %d with value %d, priority %025d, numOfEntities %d\n",
		spaces, level, n.annotationValue, n.priority, n.treeSize)

	left := n.ix.getNodeAt(n.left)
	if left != nil {
		fmt.Printf("%s < (%s)\n", spaces, n.left.Hex())
		left.print(level + 1)
	}

	right := n.ix.getNodeAt(n.right)
	if right != nil {
		fmt.Printf("%s > (%s)\n", spaces, n.right.Hex())
		right.print(level + 1)
	}
}

type ImmutableIndex interface {
	GetRootHash() common.Hash

	GetRootNode() *Node

	// IterateFromTo iterates over all the entities in this AnnotationIndex that
	// have values for this annotation in the closed interval [from, to].
	// We don't use the standard half-open interval here, because it makes more sense
	// for range queries to include the final value as well.
	IterateFromTo(from *uint64, to *uint64) iter.Seq[common.Hash]

	// Size returns the number of entities in the tree, O(1)
	Size() uint64

	// Depth calculates the depth of the tree, O(n)
	Depth() uint64

	// Print prints out the tree, O(n)
	Print()
}

type Index interface {
	ImmutableIndex

	Add(value uint64, entityKeys ...common.Hash) error

	Delete(value uint64, entityKeys ...common.Hash) error
}

type annotationIndex struct {
	db            storageutil.StateAccess
	annotationKey string
	rng           *rand.ChaCha8
}

func create(db storageutil.StateAccess, annotationKey string, rng *rand.ChaCha8) *annotationIndex {
	return &annotationIndex{
		db:            db,
		annotationKey: annotationKey,
		rng:           rng,
	}
}

func NewImmutable(db storageutil.StateAccess, annotationKey string) ImmutableIndex {
	return create(db, annotationKey, nil)
}

func New(db storageutil.StateAccess, annotationKey string, rngSeed common.Hash) Index {
	var rng *rand.ChaCha8 = nil

	// The zero hash is used when we don't need an RNG to be created, for instance
	// when we only want to do deletes.
	// We therefore only create an RNG when we get an actual non-zero seed.
	if rngSeed.Cmp(zeroHash) != 0 {
		rng = rand.NewChaCha8(rngSeed)
	}

	return create(db, annotationKey, rng)
}

func (ix *annotationIndex) valueToAddress(value uint64) common.Hash {
	return annotationindex.NumericAnnotationIndexKey(ix.annotationKey, value)
}

func (ix *annotationIndex) newNode(annotationValue uint64) *Node {
	priority := ix.rng.Uint64()

	valueBaseAddress := ix.valueToAddress(annotationValue)

	left := new(uint256.Int).SetBytes(valueBaseAddress.Bytes())

	right := new(uint256.Int).SetBytes(valueBaseAddress.Bytes())
	right.AddUint64(right, 1)

	entitySetKey := new(uint256.Int).SetBytes(valueBaseAddress.Bytes())
	entitySetKey.AddUint64(entitySetKey, 2)
	entitySetKeyHash := common.Hash(entitySetKey.Bytes32())

	return &Node{
		ix:              ix,
		annotationValue: annotationValue,
		priority:        priority,
		treeSize:        0,
		entitySetKey:    entitySetKeyHash,
		left:            common.Hash(left.Bytes32()),
		right:           common.Hash(right.Bytes32()),
	}
}

func (ix *annotationIndex) GetRootHash() common.Hash {
	return annotationindex.NumericAnnotationIndexRootHash(ix.annotationKey)
}

func (ix *annotationIndex) GetRootNode() *Node {
	return ix.getNodeAt(ix.GetRootHash())
}

func (ix *annotationIndex) setRootNode(n *Node) {
	ix.writeNodeTo(n, ix.GetRootHash())
}

func (ix *annotationIndex) writeNodeTo(n *Node, address common.Hash) {
	if n == nil {
		ix.erase(address)
	} else {
		n.writeTo(address)
	}
}

func (ix *annotationIndex) getNodeAt(address common.Hash) *Node {
	valueHash := ix.db.GetState(storageutil.GolemDBAddress, address)

	if valueHash == zeroHash {
		return nil
	}

	annotationValue := new(uint256.Int).SetBytes32(valueHash.Bytes()).Uint64()
	valueBaseAddress := ix.valueToAddress(annotationValue)

	packedValue := new(uint256.Int).SetBytes32(valueHash.Bytes())
	packedValue.Rsh(packedValue, 64)
	priority := packedValue.Uint64()

	packedValue.Rsh(packedValue, 64)
	numOfEntities := packedValue.Uint64()

	left := new(uint256.Int).SetBytes(valueBaseAddress.Bytes())

	right := new(uint256.Int).SetBytes(valueBaseAddress.Bytes())
	right.AddUint64(right, 1)

	entitySetKey := new(uint256.Int).SetBytes(valueBaseAddress.Bytes())
	entitySetKey.AddUint64(entitySetKey, 2)

	return &Node{
		ix:              ix,
		annotationValue: annotationValue,
		priority:        priority,
		treeSize:        numOfEntities,
		left:            common.Hash(left.Bytes32()),
		right:           common.Hash(right.Bytes32()),
		entitySetKey:    common.Hash(entitySetKey.Bytes32()),
	}
}

func (ix *annotationIndex) erase(hash common.Hash) {
	ix.db.SetState(storageutil.GolemDBAddress, hash, zeroHash)
}

func (ix *annotationIndex) Add(value uint64, entityKeys ...common.Hash) error {
	if ix.rng == nil {
		return fmt.Errorf("trying to add a node using an AnnotationIndex that doesn't have an RNG")
	}

	var doAdd func(node *Node, value uint64, entityKey common.Hash) (*Node, error)
	doAdd = func(node *Node, value uint64, entityKey common.Hash) (*Node, error) {
		// We descended into a subtree that doesn't exist,
		// so we create a new node.
		if node == nil {
			newNode := ix.newNode(value)
			err := newNode.addEntity(entityKey)
			if err != nil {
				return nil, fmt.Errorf("error adding entity to index: %w", err)
			}
			return newNode, nil
		}

		switch cmp.Compare(node.annotationValue, value) {

		case -1:
			newRight, err := doAdd(ix.getNodeAt(node.right), value, entityKey)
			if err != nil {
				return nil, fmt.Errorf("error adding entity to index: %w", err)
			}

			node.setRight(newRight)

			// We might need to rotate to the left
			if node.priority > newRight.priority {
				newNode, err := node.rotateLeft()
				if err != nil {
					return nil, fmt.Errorf("error while adding entity to tree: %w", err)
				}
				return newNode, nil
			}
			return node, nil

		case 1:
			newLeft, err := doAdd(ix.getNodeAt(node.left), value, entityKey)
			if err != nil {
				return nil, fmt.Errorf("error adding entity to index: %w", err)
			}

			node.setLeft(newLeft)

			// We might need to rotate to the right
			if node.priority > newLeft.priority {
				newNode, err := node.rotateRight()
				if err != nil {
					return nil, fmt.Errorf("error while adding entity to tree: %w", err)
				}
				return newNode, nil
			}
			return node, nil

		default:
			// We found an exact match
			err := node.addEntity(entityKey)
			if err != nil {
				return nil, fmt.Errorf("error adding entity to index: %w", err)
			}
			return node, nil
		}
	}

	root := ix.GetRootNode()
	for _, entityKey := range entityKeys {
		newRoot, err := doAdd(root, value, entityKey)
		if err != nil {
			// Write out the root we got so far, up to the error
			ix.setRootNode(root)
			return fmt.Errorf("error adding entity to index: %w", err)
		}
		root = newRoot
	}
	ix.setRootNode(root)
	return nil
}

func (ix *annotationIndex) removeNode(node *Node) (*Node, error) {
	left := ix.getNodeAt(node.left)
	right := ix.getNodeAt(node.right)

	if left != nil && right != nil {
		// The node has both left and right children, we rotate the tree and recurse
		// to end up in one of the other cases.
		// The direction of rotation is determined by the priorities, such that we
		// are guaranteed to get a valid traep back after the deletion of the node.

		if left.priority < right.priority {
			newNode, err := node.rotateRight()
			if err != nil {
				return nil, fmt.Errorf("error removing node from the tree: %w", err)
			}
			newRight, err := ix.removeNode(ix.getNodeAt(newNode.right))
			if err != nil {
				return nil, fmt.Errorf("error removing node from the tree: %w", err)
			}

			return newNode.setRight(newRight), nil
		} else {
			newNode, err := node.rotateLeft()
			if err != nil {
				return nil, fmt.Errorf("error removing node from the tree: %w", err)
			}
			newLeft, err := ix.removeNode(ix.getNodeAt(newNode.left))
			if err != nil {
				return nil, fmt.Errorf("error removing node from the tree: %w", err)
			}

			return newNode.setLeft(newLeft), nil
		}

	} else if ix.getNodeAt(node.left) != nil {
		// The node only has a left child, so we can just erase that left child from
		// the state, and then return it as the new root.
		// The caller of this method is responsible to write the new root to the correct
		// location in the state.
		ix.erase(node.left)
		return left, nil
	} else if ix.getNodeAt(node.right) != nil {
		// Same as above but now we only have a right child.
		ix.erase(node.right)
		return right, nil
	} else {
		// We don't have any children, so just erase the node and return nil.
		return nil, nil
	}
}

func (ix *annotationIndex) Delete(value uint64, entityKeys ...common.Hash) error {

	var doDelete func(node *Node, value uint64, entityKey common.Hash) (*Node, error)
	doDelete = func(node *Node, value uint64, entityKey common.Hash) (*Node, error) {
		// We descended into a subtree that doesn't exist,
		// so the index doesn't contain the value..
		if node == nil {
			return nil, nil
		}

		switch cmp.Compare(node.annotationValue, value) {

		case -1:
			newRight, err := doDelete(ix.getNodeAt(node.right), value, entityKey)
			if err != nil {
				return nil, fmt.Errorf("error removing entity from index: %w", err)
			}
			return node.setRight(newRight), nil

		case 1:
			newLeft, err := doDelete(ix.getNodeAt(node.left), value, entityKey)
			if err != nil {
				return nil, fmt.Errorf("error removing entity from index: %w", err)
			}
			return node.setLeft(newLeft), nil

		default:
			// We found the node for this value
			err := node.removeEntity(entityKey)
			if err != nil {
				return nil, fmt.Errorf("error removing entity from index: %w", err)
			}

			// If the node became empty, then we need to remove it
			if node.numberOfEntities() == 0 {
				newNode, err := ix.removeNode(node)
				if err != nil {
					return nil, fmt.Errorf("error removing entity from index: %w", err)
				}
				return newNode, nil
			}

			return node, nil
		}
	}

	root := ix.GetRootNode()
	for _, entityKey := range entityKeys {
		newRoot, err := doDelete(root, value, entityKey)
		if err != nil {
			// Write out the root we got so far, up to the error
			ix.setRootNode(root)
			return fmt.Errorf("error removing entity from index: %w", err)
		}
		if newRoot == nil {
			ix.setRootNode(newRoot)
			// The tree is empty, so there's nothing more to delete
			return nil
		}
		root = newRoot
	}
	ix.setRootNode(root)
	return nil
}

// IterateFromTo iterates over all the entities in this AnnotationIndex that
// have values for this annotation in the closed interval [from, to].
// We don't use the standard half-open interval here, because it makes more sense
// for range queries to include the final value as well.
func (ix *annotationIndex) IterateFromTo(fromPointer *uint64, toPointer *uint64) iter.Seq[common.Hash] {
	from := uint64(0)
	if fromPointer != nil {
		from = *fromPointer
	}

	to := uint64(math.MaxUint64)
	if toPointer != nil {
		to = *toPointer
	}

	var doIterate func(node *Node, yield func(common.Hash) bool) bool
	doIterate = func(node *Node, yield func(common.Hash) bool) bool {

		finished := false
		yield_ := func(hash common.Hash) bool {
			finished = !yield(hash)
			return !finished
		}

		if node != nil && !finished {
			switch cmp.Compare(node.annotationValue, from) {
			case 0:
				keyset.Iterate(ix.db, node.entitySetKey)(yield_)

				if !finished {
					finished = !doIterate(ix.getNodeAt(node.right), yield)
				}

			case 1:
				finished = !doIterate(ix.getNodeAt(node.left), yield)

				if node.annotationValue <= to {
					if !finished {
						keyset.Iterate(ix.db, node.entitySetKey)(yield_)
					}
					if !finished {
						finished = !doIterate(ix.getNodeAt(node.right), yield)
					}
				}

			case -1:
				finished = !doIterate(ix.getNodeAt(node.right), yield)
			}
		}

		return !finished
	}

	return iter.Seq[common.Hash](func(yield func(common.Hash) bool) {
		doIterate(ix.GetRootNode(), yield)
	})
}

func (ix *annotationIndex) Size() uint64 {
	rootNode := ix.GetRootNode()
	if rootNode == nil {
		return 0
	}
	return ix.GetRootNode().TreeSize()
}

// Depth calculates the depth of the tree, O(n)
func (ix *annotationIndex) Depth() uint64 {

	var depthFrom func(node *Node, level int) uint64
	depthFrom = func(node *Node, level int) uint64 {
		if node == nil {
			return 0
		}

		left := ix.getNodeAt(node.left)
		right := ix.getNodeAt(node.right)

		leftDepth := depthFrom(left, level+1)
		rightDepth := depthFrom(right, level+1)

		if leftDepth > rightDepth {
			return 1 + leftDepth
		}
		return 1 + rightDepth
	}

	return depthFrom(ix.GetRootNode(), 0)
}

// Print prints out the tree, O(n)
func (ix *annotationIndex) Print() {
	rootNode := ix.GetRootNode()
	if rootNode != nil {
		rootNode.print(0)
	} else {
		fmt.Println("empty index")
	}
}
