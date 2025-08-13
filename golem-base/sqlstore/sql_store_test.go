package sqls

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqlite/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewStore tests the NewStore function
func TestNewStore(t *testing.T) {
	tests := []struct {
		name    string
		dbFile  string
		wantErr bool
	}{
		{
			name:    "valid in-memory database",
			dbFile:  ":memory:",
			wantErr: false,
		},
		{
			name:    "valid file database",
			dbFile:  filepath.Join(t.TempDir(), "test.db"),
			wantErr: false,
		},
		{
			name:    "invalid database path",
			dbFile:  "/invalid/path/test.db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(tt.dbFile)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, store)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, store)
			require.NotNil(t, store.db)

			// Test that schema was applied by checking if entities table exists
			ctx := context.Background()
			var tableName string
			err = store.db.QueryRowContext(ctx, `
				SELECT name FROM sqlite_master 
				WHERE type='table' AND name='entities';
			`).Scan(&tableName)
			require.NoError(t, err)
			assert.Equal(t, "entities", tableName)

			// Cleanup
			err = store.Close()
			assert.NoError(t, err)
		})
	}
}

// TestNewStoreExistingSchema tests NewStore with existing schema
func TestNewStoreExistingSchema(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "existing.db")

	// Create first store to establish schema
	store1, err := NewStore(dbFile)
	require.NoError(t, err)
	require.NotNil(t, store1)
	err = store1.Close()
	require.NoError(t, err)

	// Create second store - should not re-apply schema
	store2, err := NewStore(dbFile)
	require.NoError(t, err)
	require.NotNil(t, store2)

	// Verify entities table still exists
	ctx := context.Background()
	var tableName string
	err = store2.db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='entities';
	`).Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "entities", tableName)

	err = store2.Close()
	assert.NoError(t, err)
}

// TestSQLStoreClose tests the Close method
func TestSQLStoreClose(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	require.NotNil(t, store)

	err = store.Close()
	assert.NoError(t, err)

	// Verify database is closed by attempting to use it
	err = store.db.Ping()
	assert.Error(t, err)
}

// TestGetQueries tests the GetQueries method
func TestGetQueries(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	queries := store.GetQueries()
	assert.NotNil(t, queries)
	assert.IsType(t, &sqlitegolem.Queries{}, queries)
}

// TestInsertBlockCreate tests InsertBlock with Create operations
func TestInsertBlockCreate(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	queries := store.GetQueries()

	// Set up initial processing status for parent block validation
	parentHash := "0x2222222222222222222222222222222222222222222222222222222222222222"
	err = queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "testnet",
		LastProcessedBlockNumber: 99, // Previous block number
		LastProcessedBlockHash:   parentHash,
	})
	require.NoError(t, err)

	// Create test data
	entityKey := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	ownerAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D397389CA6B9db17")

	createOp := &wal.Create{
		EntityKey:      entityKey,
		ExpiresAtBlock: 1000,
		Payload:        []byte("test payload"),
		Owner:          ownerAddr,
		StringAnnotations: []entity.StringAnnotation{
			{Key: "name", Value: "test entity"},
		},
		NumericAnnotations: []entity.NumericAnnotation{
			{Key: "level", Value: 42},
		},
	}

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     100,
			Hash:       common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{
			{Create: createOp},
		}),
	}

	// Insert block
	err = store.InsertBlock(ctx, blockWal, "testnet", logger)
	require.NoError(t, err)

	// Verify entity was inserted
	entity, err := queries.GetEntity(ctx, entityKey.Hex())
	require.NoError(t, err)
	// Note: GetEntityRow doesn't include Key field, so we verify other fields
	assert.Equal(t, int64(1000), entity.ExpiresAt)
	assert.Equal(t, []byte("test payload"), entity.Payload)
	assert.Equal(t, ownerAddr.Hex(), entity.OwnerAddress)

	// Verify string annotation
	stringAnns, err := queries.GetStringAnnotations(ctx, entityKey.Hex())
	require.NoError(t, err)
	require.Len(t, stringAnns, 1)
	assert.Equal(t, "name", stringAnns[0].AnnotationKey)
	assert.Equal(t, "test entity", stringAnns[0].Value)

	// Verify numeric annotation
	numericAnns, err := queries.GetNumericAnnotations(ctx, entityKey.Hex())
	require.NoError(t, err)
	require.Len(t, numericAnns, 1)
	assert.Equal(t, "level", numericAnns[0].AnnotationKey)
	assert.Equal(t, int64(42), numericAnns[0].Value)
}

// TestInsertBlockUpdate tests InsertBlock with Update operations
func TestInsertBlockUpdate(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	queries := store.GetQueries()

	// Set up initial processing status for parent block validation
	parentHash := "0x4444444444444444444444444444444444444444444444444444444444444444"
	err = queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "update-test",
		LastProcessedBlockNumber: 199, // Previous block number
		LastProcessedBlockHash:   parentHash,
	})
	require.NoError(t, err)

	entityKey := common.HexToHash("0xaaabbbcccdddeeefffaaabbbcccdddeeefffaaabbbcccdddeeefffaaabbbcccdd")
	ownerAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D397389CA6B9db17")

	// Insert initial entity
	err = queries.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
		Key:          entityKey.Hex(),
		ExpiresAt:    500,
		Payload:      []byte("original payload"),
		OwnerAddress: ownerAddr.Hex(),
	})
	require.NoError(t, err)

	// Create update operation
	updateOp := &wal.Update{
		EntityKey:      entityKey,
		ExpiresAtBlock: 1500,
		Payload:        []byte("updated payload"),
		StringAnnotations: []entity.StringAnnotation{
			{Key: "status", Value: "updated"},
		},
		NumericAnnotations: []entity.NumericAnnotation{
			{Key: "version", Value: 2},
		},
	}

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     200,
			Hash:       common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{
			{Update: updateOp},
		}),
	}

	// Insert block with update
	err = store.InsertBlock(ctx, blockWal, "update-test", logger)
	require.NoError(t, err)

	// Verify entity was updated
	entity, err := queries.GetEntity(ctx, entityKey.Hex())
	require.NoError(t, err)
	assert.Equal(t, int64(1500), entity.ExpiresAt)
	assert.Equal(t, []byte("updated payload"), entity.Payload)
	assert.Equal(t, ownerAddr.Hex(), entity.OwnerAddress) // Owner should remain the same
}

// TestInsertBlockDelete tests InsertBlock with Delete operations
func TestInsertBlockDelete(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	queries := store.GetQueries()

	// Set up initial processing status for parent block validation
	parentHash := "0x6666666666666666666666666666666666666666666666666666666666666666"
	setupTestProcessingStatus(t, queries, ctx, "delete-test", 299, parentHash)

	entityKey := common.HexToHash("0xdddeeefff000111222333444555666777888999aaabbbcccdddeeefff000111")
	ownerAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D397389CA6B9db17")

	// Insert initial entity
	err = queries.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
		Key:          entityKey.Hex(),
		ExpiresAt:    500,
		Payload:      []byte("to be deleted"),
		OwnerAddress: ownerAddr.Hex(),
	})
	require.NoError(t, err)

	// Add annotations
	err = queries.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
		EntityKey:     entityKey.Hex(),
		AnnotationKey: "type",
		Value:         "temporary",
	})
	require.NoError(t, err)

	err = queries.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
		EntityKey:     entityKey.Hex(),
		AnnotationKey: "count",
		Value:         5,
	})
	require.NoError(t, err)

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     300,
			Hash:       common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{
			{Delete: &entityKey},
		}),
	}

	// Insert block with delete
	err = store.InsertBlock(ctx, blockWal, "delete-test", logger)
	require.NoError(t, err)

	// Verify entity was deleted
	_, err = queries.GetEntity(ctx, entityKey.Hex())
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)

	// Verify annotations were deleted
	stringAnns, err := queries.GetStringAnnotations(ctx, entityKey.Hex())
	require.NoError(t, err)
	assert.Empty(t, stringAnns)

	numericAnns, err := queries.GetNumericAnnotations(ctx, entityKey.Hex())
	require.NoError(t, err)
	assert.Empty(t, numericAnns)
}

// TestInsertBlockExtend tests InsertBlock with Extend operations
func TestInsertBlockExtend(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	queries := store.GetQueries()

	// Set up initial processing status for parent block validation
	parentHash := "0x8888888888888888888888888888888888888888888888888888888888888888"
	setupTestProcessingStatus(t, queries, ctx, "extend-test", 399, parentHash)

	entityKey := common.HexToHash("0xfffeeeddccbbaaffeeeddccbbaaffeeeddccbbaaffeeeddccbbaaffeeeddccbb")
	ownerAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D397389CA6B9db17")

	// Insert initial entity
	err = queries.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
		Key:          entityKey.Hex(),
		ExpiresAt:    500,
		Payload:      []byte("test payload"),
		OwnerAddress: ownerAddr.Hex(),
	})
	require.NoError(t, err)

	// Create extend operation
	extendOp := &wal.ExtendBTL{
		EntityKey:    entityKey,
		OldExpiresAt: 500,
		NewExpiresAt: 1000,
	}

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     400,
			Hash:       common.HexToHash("0x7777777777777777777777777777777777777777777777777777777777777777"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{
			{Extend: extendOp},
		}),
	}

	// Insert block with extend
	err = store.InsertBlock(ctx, blockWal, "extend-test", logger)
	require.NoError(t, err)

	// Verify entity expiration was extended
	entity, err := queries.GetEntity(ctx, entityKey.Hex())
	require.NoError(t, err)
	assert.Equal(t, int64(1000), entity.ExpiresAt)
	assert.Equal(t, []byte("test payload"), entity.Payload) // Other fields unchanged
}

// TestInsertBlockMultipleOperations tests InsertBlock with multiple operations
func TestInsertBlockMultipleOperations(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	queries := store.GetQueries()

	// Set up initial processing status for parent block validation
	parentHash := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	setupTestProcessingStatus(t, queries, ctx, "multi-test", 499, parentHash)

	entityKey1 := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	entityKey2 := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	ownerAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D397389CA6B9db17")

	operations := []wal.Operation{
		{
			Create: &wal.Create{
				EntityKey:      entityKey1,
				ExpiresAtBlock: 1000,
				Payload:        []byte("entity 1"),
				Owner:          ownerAddr,
			},
		},
		{
			Create: &wal.Create{
				EntityKey:      entityKey2,
				ExpiresAtBlock: 2000,
				Payload:        []byte("entity 2"),
				Owner:          ownerAddr,
			},
		},
		{
			Delete: &entityKey1,
		},
	}

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     500,
			Hash:       common.HexToHash("0x9999999999999999999999999999999999999999999999999999999999999999"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator(operations),
	}

	// Insert block with multiple operations
	err = store.InsertBlock(ctx, blockWal, "multi-test", logger)
	require.NoError(t, err)

	// Verify entity1 was deleted
	_, err = queries.GetEntity(ctx, entityKey1.Hex())
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)

	// Verify entity2 still exists
	entity2, err := queries.GetEntity(ctx, entityKey2.Hex())
	require.NoError(t, err)
	// Note: GetEntityRow doesn't include Key field, so we verify other fields
	assert.Equal(t, []byte("entity 2"), entity2.Payload)
}

// TestInsertBlockTransactionRollback tests that failed operations rollback the transaction
func TestInsertBlockTransactionRollback(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a block with wrong parent hash to trigger validation failure
	entityKey := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     600,
			Hash:       common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			ParentHash: common.HexToHash("0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{
			{
				Update: &wal.Update{
					EntityKey:      entityKey,
					ExpiresAtBlock: 1000,
					Payload:        []byte("should fail"),
				},
			},
		}),
	}

	// Insert block should fail due to missing processing status
	err = store.InsertBlock(ctx, blockWal, "rollback-test", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get processing status")

	// Verify no partial state was committed
	queries := store.GetQueries()
	_, err = queries.GetEntity(ctx, entityKey.Hex())
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}

// TestInsertBlockProcessingStatus tests that processing status is updated
func TestInsertBlockProcessingStatus(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "processing-status.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})) // Reduce log noise
	queries := store.GetQueries()

	// Set up initial processing status with correct parent block relationship
	parentHash := "0x6666666666666666666666666666666666666666666666666666666666666666"
	setupTestProcessingStatus(t, queries, ctx, "processing-test", 699, parentHash)

	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     700,
			Hash:       common.HexToHash("0x7777777777777777777777777777777777777777777777777777777777777777"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	// Insert block
	err = store.InsertBlock(ctx, blockWal, "processing-test", logger)
	require.NoError(t, err)

	// Verify processing status was updated
	status, err := queries.GetProcessingStatus(ctx, "processing-test")
	require.NoError(t, err)
	assert.Equal(t, int64(700), status.LastProcessedBlockNumber)
	assert.Equal(t, "0x7777777777777777777777777777777777777777777777777777777777777777", status.LastProcessedBlockHash)
}

// TestProcessingStatusDirectUpdate tests the processing status update directly
func TestProcessingStatusDirectUpdate(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "direct-update.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	queries := store.GetQueries()

	// Insert initial processing status record
	err = queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "direct-test",
		LastProcessedBlockNumber: 0,
		LastProcessedBlockHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)

	// Verify initial state
	status, err := queries.GetProcessingStatus(ctx, "direct-test")
	require.NoError(t, err)
	assert.Equal(t, int64(0), status.LastProcessedBlockNumber)
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", status.LastProcessedBlockHash)

	// Update processing status
	err = queries.UpdateProcessingStatus(ctx, sqlitegolem.UpdateProcessingStatusParams{
		Network:                  "direct-test",
		LastProcessedBlockNumber: 123,
		LastProcessedBlockHash:   "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	})
	require.NoError(t, err)

	// Verify update worked
	status, err = queries.GetProcessingStatus(ctx, "direct-test")
	require.NoError(t, err)
	assert.Equal(t, int64(123), status.LastProcessedBlockNumber)
	assert.Equal(t, "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", status.LastProcessedBlockHash)
}

// TestTransactionProcessingStatusUpdate tests processing status update within a transaction
func TestTransactionProcessingStatusUpdate(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "tx-update.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Insert initial processing status using autocommit
	autocommitQueries := store.GetQueries()
	err = autocommitQueries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "tx-test",
		LastProcessedBlockNumber: 0,
		LastProcessedBlockHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)

	// Start transaction
	tx, err := store.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Use transaction queries
	txQueries := sqlitegolem.New(tx)

	// Update within transaction
	err = txQueries.UpdateProcessingStatus(ctx, sqlitegolem.UpdateProcessingStatusParams{
		Network:                  "tx-test",
		LastProcessedBlockNumber: 500,
		LastProcessedBlockHash:   "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
	})
	require.NoError(t, err)

	// Commit transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify update worked using autocommit
	status, err := autocommitQueries.GetProcessingStatus(ctx, "tx-test")
	require.NoError(t, err)
	assert.Equal(t, int64(500), status.LastProcessedBlockNumber)
	assert.Equal(t, "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", status.LastProcessedBlockHash)
}

// TestInsertBlockParentHashValidation tests parent hash validation
func TestInsertBlockParentHashValidation(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "parent-validation.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Set up initial processing status
	correctParentHash := "0x1010101010101010101010101010101010101010101010101010101010101010"
	setupTestProcessingStatus(t, queries, ctx, "validation-test", 49, correctParentHash)

	// Test with correct parent hash - should succeed
	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     50,
			Hash:       common.HexToHash("0x2020202020202020202020202020202020202020202020202020202020202020"),
			ParentHash: common.HexToHash(correctParentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWal, "validation-test", logger)
	assert.NoError(t, err)

	// Test with wrong parent hash - should fail
	blockWalWrongParent := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     51,
			Hash:       common.HexToHash("0x3030303030303030303030303030303030303030303030303030303030303030"),
			ParentHash: common.HexToHash("0xwronghash000000000000000000000000000000000000000000000000000"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWalWrongParent, "validation-test", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent block hash mismatch")
}

// TestInsertBlockSequenceValidation tests block number sequence validation
func TestInsertBlockSequenceValidation(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "sequence-validation.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Set up initial processing status
	parentHash := "0x4040404040404040404040404040404040404040404040404040404040404040"
	setupTestProcessingStatus(t, queries, ctx, "sequence-test", 99, parentHash)

	// Test with wrong block number - should fail
	blockWal := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     102, // Should be 100
			Hash:       common.HexToHash("0x5050505050505050505050505050505050505050505050505050505050505050"),
			ParentHash: common.HexToHash(parentHash),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWal, "sequence-test", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block number sequence error")
}

// TestInsertBlockGenesisBlock tests that genesis block (number 0) skips validation
func TestInsertBlockGenesisBlock(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "genesis.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Insert initial processing status for genesis
	err = queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "genesis-test",
		LastProcessedBlockNumber: -1, // Before genesis
		LastProcessedBlockHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)

	// Genesis block should succeed without parent validation
	genesisBlock := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     0, // Genesis block
			Hash:       common.HexToHash("0x6060606060606060606060606060606060606060606060606060606060606060"),
			ParentHash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, genesisBlock, "genesis-test", logger)
	assert.NoError(t, err)
}

// TestInsertBlockSingleNetworkConstraint tests that only one network can exist in the database
func TestInsertBlockSingleNetworkConstraint(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "single-network.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Set up first network
	setupTestProcessingStatus(t, queries, ctx, "network1", 99, "0x1010101010101010101010101010101010101010101010101010101010101010")

	// Insert block for first network - should succeed
	blockWal1 := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     100,
			Hash:       common.HexToHash("0x2020202020202020202020202020202020202020202020202020202020202020"),
			ParentHash: common.HexToHash("0x1010101010101010101010101010101010101010101010101010101010101010"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWal1, "network1", logger)
	assert.NoError(t, err)

	// Try to insert block for second network - should fail
	blockWal2 := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     0, // Genesis block for second network
			Hash:       common.HexToHash("0x3030303030303030303030303030303030303030303030303030303030303030"),
			ParentHash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWal2, "network2", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot add network network2: database already contains 1 network(s), only one network is allowed")
}

// TestInsertBlockSingleNetworkAllowSameNetwork tests that the same network can continue to be used
func TestInsertBlockSingleNetworkAllowSameNetwork(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "same-network.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Set up network
	setupTestProcessingStatus(t, queries, ctx, "mainnet", 99, "0x4040404040404040404040404040404040404040404040404040404040404040")

	// Insert first block - should succeed
	blockWal1 := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     100,
			Hash:       common.HexToHash("0x5050505050505050505050505050505050505050505050505050505050505050"),
			ParentHash: common.HexToHash("0x4040404040404040404040404040404040404040404040404040404040404040"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWal1, "mainnet", logger)
	assert.NoError(t, err)

	// Insert second block for same network - should succeed
	blockWal2 := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     101,
			Hash:       common.HexToHash("0x6060606060606060606060606060606060606060606060606060606060606060"),
			ParentHash: common.HexToHash("0x5050505050505050505050505050505050505050505050505050505050505050"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, blockWal2, "mainnet", logger)
	assert.NoError(t, err)
}

// TestInsertBlockFirstNetworkAllowed tests that the first network is allowed in an empty database
func TestInsertBlockFirstNetworkAllowed(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "first-network.db"))
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Insert initial processing status for the first network
	err = queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "testnet",
		LastProcessedBlockNumber: -1,
		LastProcessedBlockHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)

	// Insert genesis block for first network - should succeed
	genesisBlock := wal.BlockWal{
		BlockInfo: wal.BlockInfo{
			Number:     0,
			Hash:       common.HexToHash("0x7070707070707070707070707070707070707070707070707070707070707070"),
			ParentHash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		},
		OperationsIterator: createMockIterator([]wal.Operation{}),
	}

	err = store.InsertBlock(ctx, genesisBlock, "testnet", logger)
	assert.NoError(t, err)
}

// setupTestProcessingStatus sets up initial processing status for a test
func setupTestProcessingStatus(t *testing.T, queries *sqlitegolem.Queries, ctx context.Context, network string, blockNumber int64, blockHash string) {
	err := queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  network,
		LastProcessedBlockNumber: blockNumber,
		LastProcessedBlockHash:   blockHash,
	})
	require.NoError(t, err)
}

// createMockIterator creates a mock BlockOperationsIterator for testing
func createMockIterator(operations []wal.Operation) wal.BlockOperationsIterator {
	return func(yield func(operation wal.Operation, err error) bool) {
		for _, op := range operations {
			if !yield(op, nil) {
				return
			}
		}
	}
}

// BenchmarkInsertBlock benchmarks the InsertBlock method
func BenchmarkInsertBlock(b *testing.B) {
	store, err := NewStore(filepath.Join(b.TempDir(), "benchmark.db"))
	require.NoError(b, err)
	defer store.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queries := store.GetQueries()

	// Set up initial processing status for benchmarking
	err = queries.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
		Network:                  "benchmark",
		LastProcessedBlockNumber: -1,
		LastProcessedBlockHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(b, err)

	entityKey := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	ownerAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D397389CA6B9db17")

	createOp := &wal.Create{
		EntityKey:      entityKey,
		ExpiresAtBlock: 1000,
		Payload:        []byte("benchmark payload"),
		Owner:          ownerAddr,
		StringAnnotations: []entity.StringAnnotation{
			{Key: "name", Value: "benchmark entity"},
		},
		NumericAnnotations: []entity.NumericAnnotation{
			{Key: "level", Value: 42},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// For benchmark, we need to create proper parent-child relationships
		var parentHash common.Hash
		if i == 0 {
			parentHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
		} else {
			parentHash = common.BytesToHash([]byte{byte(i - 1)})
		}

		blockWal := wal.BlockWal{
			BlockInfo: wal.BlockInfo{
				Number:     uint64(i),
				Hash:       common.BytesToHash([]byte{byte(i)}),
				ParentHash: parentHash,
			},
			OperationsIterator: createMockIterator([]wal.Operation{
				{Create: createOp},
			}),
		}

		err := store.InsertBlock(ctx, blockWal, "benchmark", logger)
		require.NoError(b, err)
	}
}
