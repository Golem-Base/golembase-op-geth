package hasher

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
)

func main() {
	// Example usage of SimpleMerkleTree

	// Create a test file
	testFile := "/tmp/test_merkle.dat"
	createTestFile(testFile, 1024*10) // 10KB file
	defer os.Remove(testFile)

	// Create merkle tree with 1KB chunks
	mt := NewSimpleMerkleTree(1024, testFile)

	// Build initial tree
	fmt.Println("Building initial Merkle tree...")
	if err := mt.Build(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Chunk count: %d\n", mt.ChunkCount())
	fmt.Printf("Root hash: %s\n", hex.EncodeToString(mt.Root().Bytes()))

	// Modify a chunk in the middle
	fmt.Println("\nModifying chunk 5...")
	if err := modifyFile(testFile, 5*1024, []byte("MODIFIED DATA")); err != nil {
		log.Fatal(err)
	}

	// Update only the modified chunk
	blockRanges := []BlockRange{
		{Start: 5, Length: 1}, // Update chunk 5
	}

	if err := mt.Update(blockRanges); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("New root hash: %s\n", hex.EncodeToString(mt.Root().Bytes()))

	// Truncate the file
	fmt.Println("\nTruncating file to 5KB...")
	if err := os.Truncate(testFile, 5*1024); err != nil {
		log.Fatal(err)
	}

	// Update with truncation (empty block ranges)
	if err := mt.Update(nil); err != nil {
		log.Fatal(err)
	}

	mt2 := NewSimpleMerkleTree(1024, testFile)
	if err := mt2.Build(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Root hash after truncation: %s\n", hex.EncodeToString(mt2.Root().Bytes()))

	fmt.Printf("Chunk count after truncation: %d\n", mt.ChunkCount())
	fmt.Printf("Root hash after truncation: %s\n", hex.EncodeToString(mt.Root().Bytes()))

	// Example: Multiple writes
	fmt.Println("\nSimulating multiple writes...")
	if err := modifyFile(testFile, 1024, []byte("WRITE1")); err != nil {
		log.Fatal(err)
	}
	if err := modifyFile(testFile, 3*1024, []byte("WRITE2")); err != nil {
		log.Fatal(err)
	}

	// Update multiple chunks at once
	blockRanges = []BlockRange{
		{Start: 1, Length: 1}, // Chunk 1
		{Start: 3, Length: 1}, // Chunk 3
	}

	if err := mt.Update(blockRanges); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Root hash after multiple writes: %s\n", hex.EncodeToString(mt.Root().Bytes()))
	// create a new merkle tree to calculate the root hash of the file and compare it with the previous root hash
	mt2 = NewSimpleMerkleTree(1024, testFile)
	if err := mt2.Build(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Root hash after multiple writes: %s\n", hex.EncodeToString(mt2.Root().Bytes()))

	if mt.Root() == mt2.Root() {
		fmt.Println("Root hashes are the same")
	} else {
		fmt.Println("Root hashes are different")
	}
}

// createTestFile creates a test file with random-like data
func createTestFile(path string, size int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write some pattern data
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	_, err = f.Write(data)
	return err
}

// modifyFile writes data at a specific offset
func modifyFile(path string, offset int64, data []byte) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return err
	}

	_, err = f.Write(data)
	return err
}
