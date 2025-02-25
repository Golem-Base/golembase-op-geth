package walwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/wal"
)

func NewIterator(
	ctx context.Context,
	walDir string,
	nextBlockNumber uint64,
	prevBlockHash common.Hash,
) func(yield func(blockNumber uint64, err error) bool) {

	blockNumber := nextBlockNumber

	return func(yield func(blockNumber uint64, err error) bool) {

		for ctx.Err() == nil {

			filename := filepath.Join(walDir, wal.BlockNumberToFilename(blockNumber))

			f, err := os.Open(filename)
			if err != nil {
				if !yield(0, fmt.Errorf("failed to open file: %w", err)) {
					return
				}
			}
			defer f.Close()

			dec := json.NewDecoder(f)
			bi := wal.BlockInfo{}
			err = dec.Decode(&bi)

			if err != nil {
				if !yield(0, fmt.Errorf("failed to decode block: %w", err)) {
					return
				}
			}

			if bi.Number != blockNumber {
				if !yield(0, fmt.Errorf("block number mismatch: expected %d, got %d", blockNumber, bi.Number)) {
					return
				}
			}

			if bi.ParentHash != prevBlockHash {
				if !yield(0, fmt.Errorf("block hash mismatch: expected %s, got %s", prevBlockHash.Hex(), bi.Hash.Hex())) {
					return
				}
			}

			yield(bi.Number, nil)

			blockNumber = bi.Number + 1
			prevBlockHash = bi.Hash
		}
	}

}
