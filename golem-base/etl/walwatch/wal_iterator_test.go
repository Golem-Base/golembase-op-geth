package walwatch_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/etl/walwatch"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func writeWal(
	dir string,
	blockInfo wal.BlockInfo,
	operations []wal.Operation,
) (err error) {

	f, err := os.Create(filepath.Join(dir, wal.BlockNumberToFilename(blockInfo.Number)+".temp"))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	err = enc.Encode(blockInfo)
	if err != nil {
		return fmt.Errorf("failed to encode block info: %w", err)
	}

	for _, operation := range operations {
		err = enc.Encode(operation)
		if err != nil {
			return fmt.Errorf("failed to encode operation: %w", err)
		}
	}

	err = f.Close()
	if err != nil {
		return err
	}

	err = os.Rename(filepath.Join(dir, wal.BlockNumberToFilename(blockInfo.Number)+".temp"), filepath.Join(dir, wal.BlockNumberToFilename(blockInfo.Number)))
	if err != nil {
		return err
	}

	return nil
}

func TestWalIterator(t *testing.T) {

	t.Run("should iterate over one block", func(t *testing.T) {

		log.SetDefault(log.NewLogger(slog.NewTextHandler(os.Stdout, nil)))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		td := t.TempDir()

		// ops := make(chan []wal.Operation)

		err := writeWal(td,
			wal.BlockInfo{
				Number:     0,
				Hash:       common.HexToHash("0x1"),
				ParentHash: common.Hash{},
			},
			[]wal.Operation{
				{
					Create: &wal.Create{
						EntityKey:      common.HexToHash("0x1"),
						Payload:        []byte{1, 2, 3},
						ExpiresAtBlock: 100,
					},
				},
			},
		)
		require.NoError(t, err)

		for block, err := range walwatch.NewIterator(ctx, td, 0, common.Hash{}) {
			require.Equal(t, block, uint64(0))
			require.NoError(t, err)
			cancel()
		}
	})

	t.Run("should iterate over two blocks", func(t *testing.T) {

		log.SetDefault(log.NewLogger(slog.NewTextHandler(os.Stdout, nil)))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		td := t.TempDir()

		// ops := make(chan []wal.Operation)

		err := writeWal(td,
			wal.BlockInfo{
				Number:     0,
				Hash:       common.HexToHash("0x1"),
				ParentHash: common.Hash{},
			},
			[]wal.Operation{
				{
					Create: &wal.Create{
						EntityKey:      common.HexToHash("0x1"),
						Payload:        []byte{1, 2, 3},
						ExpiresAtBlock: 100,
					},
				},
			},
		)
		require.NoError(t, err)

		err = writeWal(td,
			wal.BlockInfo{
				Number:     1,
				Hash:       common.HexToHash("0x2"),
				ParentHash: common.HexToHash("0x1"),
			},
			[]wal.Operation{
				{
					Create: &wal.Create{
						EntityKey:      common.HexToHash("0x1"),
						Payload:        []byte{1, 2, 3},
						ExpiresAtBlock: 100,
					},
				},
			},
		)
		require.NoError(t, err)

		expectedBlocks := []uint64{0, 1}

		for block, err := range walwatch.NewIterator(ctx, td, 0, common.Hash{}) {
			require.Equal(t, expectedBlocks[0], block)
			require.NoError(t, err)
			expectedBlocks = expectedBlocks[1:]
			if len(expectedBlocks) == 0 {
				cancel()
			}
		}
	})

}
