package hasher

import (
	"crypto/sha256"
	"io"
	"math"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

// BlockRange represents a range of chunks to update
type BlockRange struct {
	Start  int64 // Starting chunk index
	Length int64 // Number of chunks
}

// SimpleMerkleTree is a wrapper for chunked file hashing
// using a Merkle tree structure
type SimpleMerkleTree struct {
	chunkSize   int
	chunkHashes [][]byte // SHA256 digests of each chunk
	fullPath    string
}

// NewSimpleMerkleTree creates a new SimpleMerkleTree with the given chunk size
func NewSimpleMerkleTree(chunkSize int, fullPath string) *SimpleMerkleTree {
	return &SimpleMerkleTree{
		chunkSize:   chunkSize,
		chunkHashes: make([][]byte, 0),
		fullPath:    fullPath,
	}
}

// Build reads the file in chunks, computes SHA-256 of each,
// and builds the Merkle tree
func (mt *SimpleMerkleTree) Build() error {
	// Get file size
	info, err := os.Stat(mt.fullPath)
	if err != nil {
		return err
	}

	size := info.Size()
	count := int(math.Ceil(float64(size) / float64(mt.chunkSize)))

	// Clear existing hashes
	mt.chunkHashes = make([][]byte, 0, count)

	// Open file for reading
	f, err := os.Open(mt.fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read and hash each chunk
	buffer := make([]byte, mt.chunkSize)
	for i := 0; i < count; i++ {
		n, err := f.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}

		// Hash the chunk
		hash := sha256.Sum256(buffer[:n])
		mt.chunkHashes = append(mt.chunkHashes, hash[:])
	}

	return nil
}

// Update applies chunk-level updates or truncates, then rebuilds the tree
func (mt *SimpleMerkleTree) Update(blockRanges []BlockRange) error {
	log.Debug("Updating Merkle tree", "blockRanges", blockRanges)
	// 1) Re-digest any ranges marked as "dirty"
	if len(blockRanges) > 0 {
		f, err := os.Open(mt.fullPath)
		if err != nil {
			return err
		}
		defer f.Close()

		buffer := make([]byte, mt.chunkSize)

		for _, br := range blockRanges {
			for idx := br.Start; idx < br.Start+br.Length; idx++ {
				if idx > int64(len(mt.chunkHashes)) {
					break
				}
				// Seek to chunk position
				offset := int64(idx) * int64(mt.chunkSize)
				if _, err := f.Seek(offset, io.SeekStart); err != nil {
					return err
				}

				// Read chunk
				n, err := f.Read(buffer)
				if err != nil && err != io.EOF {
					return err
				}

				// Hash the chunk
				hash := sha256.Sum256(buffer[:n])

				if idx < int64(len(mt.chunkHashes)) {
					// Overwrite existing chunk digest
					mt.chunkHashes[idx] = hash[:]
				} else {
					// Writing past EOF; append new chunk digest
					mt.chunkHashes = append(mt.chunkHashes, hash[:])
				}
			}
		}
	}

	// 2) Handle truncation: drop any chunk digests past current EOF
	info, err := os.Stat(mt.fullPath)
	if err != nil {
		return err
	}

	sizeBytes := info.Size()
	expectedChunks := int(math.Ceil(float64(sizeBytes) / float64(mt.chunkSize)))

	if len(mt.chunkHashes) > expectedChunks {
		// Prune trailing entries
		mt.chunkHashes = mt.chunkHashes[:expectedChunks]

		// Re-hash the last chunk (may be partial)
		if expectedChunks > 0 {
			f, err := os.Open(mt.fullPath)
			if err != nil {
				return err
			}
			defer f.Close()

			offset := int64(expectedChunks-1) * int64(mt.chunkSize)
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return err
			}

			buffer := make([]byte, mt.chunkSize)
			n, err := f.Read(buffer)
			if err != nil && err != io.EOF {
				return err
			}

			hash := sha256.Sum256(buffer[:n])
			mt.chunkHashes[expectedChunks-1] = hash[:]
		}
	}

	return nil
}

// Root returns the current Merkle root as raw bytes
func (mt *SimpleMerkleTree) Root() []byte {
	if len(mt.chunkHashes) == 0 {
		return []byte{}
	}

	log.Trace("Building Merkle tree", "chunkHashes", mt.chunkHashes)

	return mt.buildMerkleRoot(mt.chunkHashes)
}

// buildMerkleRoot builds a Merkle tree from the chunk hashes and returns the root
func (mt *SimpleMerkleTree) buildMerkleRoot(hashes [][]byte) []byte {
	if len(hashes) == 0 {
		return []byte{}
	}

	if len(hashes) == 1 {
		return hashes[0]
	}

	// Build next level of the tree
	var nextLevel [][]byte

	for i := 0; i < len(hashes); i += 2 {
		if i+1 < len(hashes) {
			// Combine two hashes
			combined := append(hashes[i], hashes[i+1]...)
			hash := sha256.Sum256(combined)
			nextLevel = append(nextLevel, hash[:])
		} else {
			// Odd number of hashes, carry forward the last one
			combined := append(hashes[i], hashes[i]...)
			hash := sha256.Sum256(combined)
			nextLevel = append(nextLevel, hash[:])
		}
	}

	// Recursively build the tree
	return mt.buildMerkleRoot(nextLevel)
}

// GetChunkHashes returns a copy of the current chunk hashes
func (mt *SimpleMerkleTree) GetChunkHashes() [][]byte {
	result := make([][]byte, len(mt.chunkHashes))
	for i, hash := range mt.chunkHashes {
		result[i] = make([]byte, len(hash))
		copy(result[i], hash)
	}
	return result
}

// ChunkCount returns the number of chunks currently tracked
func (mt *SimpleMerkleTree) ChunkCount() int {
	return len(mt.chunkHashes)
}

// Copy creates a deep copy of the SimpleMerkleTree
func (mt *SimpleMerkleTree) Copy() *SimpleMerkleTree {
	// Create new instance with same chunk size and path
	copiedTree := &SimpleMerkleTree{
		chunkSize:   mt.chunkSize,
		fullPath:    mt.fullPath,
		chunkHashes: make([][]byte, len(mt.chunkHashes)),
	}

	// Deep copy the chunk hashes
	for i, hash := range mt.chunkHashes {
		copiedTree.chunkHashes[i] = make([]byte, len(hash))
		copy(copiedTree.chunkHashes[i], hash)
	}

	return copiedTree
}
