package sqlstore

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	_ "github.com/mattn/go-sqlite3"
)

const entitiesSchemaVersion = uint64(2)

type BlockWal struct {
	BlockInfo  BlockInfo
	Operations []Operation
}
type BlockInfo struct {
	Number     uint64      `json:"number,string"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type Operation struct {
	Create *Create    `json:"create,omitempty"`
	Update *Update    `json:"update,omitempty"`
	Delete *Delete    `json:"delete,omitempty"`
	Extend *ExtendBTL `json:"extend,omitempty"`
}

type Create struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
	Owner              common.Address             `json:"owner"`
	TransactionIndex   uint64                     `json:"txIndex"`
	OperationIndex     uint64                     `json:"opIndex"`
}

type Update struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
	TransactionIndex   uint64                     `json:"txIndex"`
	OperationIndex     uint64                     `json:"opIndex"`
}

type ExtendBTL struct {
	EntityKey        common.Hash `json:"entityKey"`
	OldExpiresAt     uint64      `json:"oldExpiresAt"`
	NewExpiresAt     uint64      `json:"newExpiresAt"`
	TransactionIndex uint64      `json:"txIndex"`
	OperationIndex   uint64      `json:"opIndex"`
}

type Delete struct {
	EntityKey        common.Hash `json:"entityKey"`
	TransactionIndex uint64      `json:"txIndex"`
	OperationIndex   uint64      `json:"opIndex"`
}

// SQLStore encapsulates the SQLite SQLStore functionality
type SQLStore struct {
	db                  *sql.DB
	historicBlocksCount uint64
}

// NewStore creates a new ETL instance with database connection and schema setup
func NewStore(dbFile string, historicBlocksCount uint64) (*SQLStore, error) {
	dir := filepath.Dir(dbFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", dbFile))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Check if schema exists and apply if needed
	ctx := context.Background()

	// Check if schema is up to date
	readVersions := true
	entitiesVersion := uint64(0)

	var tableName string
	err = db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='schema_versions';
	`).Scan(&tableName)

	switch err {
	case sql.ErrNoRows:
		// In version 0, we didn't have the schema_versions table yet
		entitiesVersion = 0
		readVersions = false
		log.Warn("arkiv: no schema version info found, table missing")
	case nil:
		// The schema exists, we can read the versions from it
	default:
		// We got another error
		db.Close()
		return nil, fmt.Errorf("failed to check schema: %w", err)
	}

	if readVersions {
		err = db.QueryRowContext(
			ctx,
			`SELECT entities FROM schema_versions WHERE id = 1;`,
		).Scan(&entitiesVersion)

		switch err {
		case sql.ErrNoRows:
			entitiesVersion = 0
			log.Warn("arkiv: no schema version info found, table empty", "error", err)
		case nil:
			// We read the versions, all good
			log.Info("arkiv: schema versions read from database", "entities", entitiesVersion)
		default:
			db.Close()
			return nil, fmt.Errorf("failed to check schema: %w", err)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if entitiesVersion != entitiesSchemaVersion {
		log.Warn(
			"arkiv: entities table has an outdated schema, dropping tables",
			"existingVersion", entitiesVersion,
			"requiredVersion", entitiesSchemaVersion,
		)
		_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS entities;`)
		if err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("failed to drop entities table: %w", err)
		}
		_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS string_annotations;`)
		if err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("failed to drop string_annotations table: %w", err)
		}
		_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS numeric_annotations;`)
		if err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("failed to drop numeric_annotations table: %w", err)
		}
	}

	log.Info("arkiv: applying database schema")
	err = sqlitegolem.ApplySchemaTx(ctx, tx)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to recreate schema: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO schema_versions (id, entities) VALUES (1, ?);`,
		entitiesSchemaVersion)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to update schema versions: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to recreate schema: %w", err)
	}

	log.Info("arkiv: database ready", "entitySchemaVersion", entitiesSchemaVersion)
	return &SQLStore{
		db:                  db,
		historicBlocksCount: historicBlocksCount,
	}, nil
}

// Close closes the database connection
func (e *SQLStore) Close() error {
	return e.db.Close()
}

// GetQueries returns a new sqlitegolem.Queries instance for autocommit operations
func (e *SQLStore) GetQueries() *sqlitegolem.Queries {
	return sqlitegolem.New(e.db)
}

func (e *SQLStore) GetProcessingStatus(ctx context.Context, networkID string) (*sqlitegolem.GetProcessingStatusRow, error) {
	result, err := e.GetQueries().GetProcessingStatus(ctx, networkID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &sqlitegolem.GetProcessingStatusRow{
				LastProcessedBlockNumber: 0,
				LastProcessedBlockHash:   "",
			}, nil
		}
		return nil, err
	}
	return &result, nil
}

// GetAllEntityKeys retrieves all entity keys from the database
func (e *SQLStore) GetAllEntityKeys(ctx context.Context, block uint64) ([]common.Hash, error) {
	keys, err := e.GetQueries().GetAllEntityKeys(ctx, int64(block))
	if err != nil {
		return nil, fmt.Errorf("failed to get all entity keys: %w", err)
	}

	// Convert string keys to common.Hash
	result := make([]common.Hash, 0, len(keys))
	for _, keyHex := range keys {
		result = append(result, common.HexToHash(keyHex))
	}

	return result, nil
}

// GetEntityCount retrieves the total number of entities in the database
func (e *SQLStore) GetEntityCount(ctx context.Context, block uint64) (uint64, error) {
	count, err := e.GetQueries().GetEntityCount(ctx, int64(block))
	if err != nil {
		return 0, fmt.Errorf("failed to get entity count: %w", err)
	}

	return uint64(count), nil
}

func (e *SQLStore) SnapSyncToBlock(
	ctx context.Context,
	networkID string,
	blockNumber uint64,
	blockHash common.Hash,
	entities iter.Seq2[
		*struct {
			Key      common.Hash
			Metadata entity.EntityMetaData
			Payload  []byte
		},
		error,
	],
) (err error) {
	log.Info("snap syncing to block start", "blockNumber", blockNumber, "blockHash", blockHash.Hex())
	defer log.Info("snap syncing to block end", "blockNumber", blockNumber, "blockHash", blockHash.Hex())

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	txDB := sqlitegolem.New(tx)

	// Ensure single network constraint for snap sync
	hasNetwork, err := txDB.HasProcessingStatus(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to check if network exists: %w", err)
	}

	if !hasNetwork {
		// This is a new network, check if there are already other networks
		networkCount, err := txDB.CountNetworks(ctx)
		if err != nil {
			return fmt.Errorf("failed to count existing networks: %w", err)
		}

		if networkCount > 0 {
			return fmt.Errorf("cannot snap sync to network %s: database already contains %d network(s), only one network is allowed", networkID, networkCount)
		}

		// First network, need to insert initial processing status
		err = txDB.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
			Network:                  networkID,
			LastProcessedBlockNumber: int64(blockNumber),
			LastProcessedBlockHash:   blockHash.Hex(),
		})
		if err != nil {
			return fmt.Errorf("failed to insert initial processing status: %w", err)
		}
	}

	// Clear all existing entities, annotations for a clean snap sync
	err = txDB.DeleteAllEntities(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear entities: %w", err)
	}

	err = txDB.DeleteAllStringAnnotations(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear string annotations: %w", err)
	}

	err = txDB.DeleteAllNumericAnnotations(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear numeric annotations: %w", err)
	}

	// Insert all entities from the snapshot
	for entity, err := range entities {
		if err != nil {
			return fmt.Errorf("failed to get entity: %w", err)
		}

		// Insert the entity
		err = txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
			Key:                         entity.Key.Hex(),
			ExpiresAt:                   int64(entity.Metadata.ExpiresAtBlock),
			Payload:                     entity.Payload,
			OwnerAddress:                entity.Metadata.Owner.Hex(),
			CreatedAtBlock:              int64(entity.Metadata.CreatedAtBlock),
			LastModifiedAtBlock:         int64(entity.Metadata.LastModifiedAtBlock),
			TransactionIndexInBlock:     int64(entity.Metadata.TransactionIndex),
			OperationIndexInTransaction: int64(entity.Metadata.OperationIndex),
		})
		if err != nil {
			return fmt.Errorf("failed to insert entity %s: %w", entity.Key.Hex(), err)
		}

		// Insert string annotations
		for _, annotation := range entity.Metadata.StringAnnotations {
			err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
				EntityKey:     entity.Key.Hex(),
				AnnotationKey: annotation.Key,
				Value:         annotation.Value,
			})
			if err != nil {
				return fmt.Errorf("failed to insert string annotation for entity %s: %w", entity.Key.Hex(), err)
			}
		}

		// Insert numeric annotations
		for _, annotation := range entity.Metadata.NumericAnnotations {
			err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
				EntityKey:     entity.Key.Hex(),
				AnnotationKey: annotation.Key,
				Value:         int64(annotation.Value),
			})
			if err != nil {
				return fmt.Errorf("failed to insert numeric annotation for entity %s: %w", entity.Key.Hex(), err)
			}
		}
	}

	// Update processing status to the snap sync block
	err = txDB.UpdateProcessingStatus(ctx, sqlitegolem.UpdateProcessingStatusParams{
		Network:                  networkID,
		LastProcessedBlockNumber: int64(blockNumber),
		LastProcessedBlockHash:   blockHash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("failed to update processing status: %w", err)
	}

	return tx.Commit()
}

// InsertBlock processes a single block from the WAL and inserts it into the database
func (e *SQLStore) InsertBlock(ctx context.Context, blockWal BlockWal, networkID string) (err error) {
	log.Info("processing block", "block", blockWal.BlockInfo.Number)
	defer log.Info("processing block end", "block", blockWal.BlockInfo.Number)

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	txDB := sqlitegolem.New(tx)

	// Ensure single network constraint: check if this would create a new network
	hasNetwork, err := txDB.HasProcessingStatus(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to check if network exists: %w", err)
	}

	if !hasNetwork {
		// This is a new network, check if there are already other networks
		networkCount, err := txDB.CountNetworks(ctx)
		if err != nil {
			return fmt.Errorf("failed to count existing networks: %w", err)
		}

		if networkCount > 0 {
			return fmt.Errorf("cannot add network %s: database already contains %d network(s), only one network is allowed", networkID, networkCount)
		}

		err = txDB.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
			Network:                  networkID,
			LastProcessedBlockNumber: int64(blockWal.BlockInfo.Number - 1),
			LastProcessedBlockHash:   blockWal.BlockInfo.ParentHash.Hex(),
		})
		if err != nil {
			return fmt.Errorf("failed to insert initial processing status: %w", err)
		}
	}

	log.Info("hasNetwork", "hasNetwork", hasNetwork)

	// Check if parent block hash matches the expected value from processing status
	if blockWal.BlockInfo.Number > 1 { // Skip check for genesis block
		processingStatus, err := txDB.GetProcessingStatus(ctx, networkID)
		if err != nil {
			return fmt.Errorf("failed to get processing status: %w", err)
		}

		expectedParentHash := processingStatus.LastProcessedBlockHash
		actualParentHash := blockWal.BlockInfo.ParentHash.Hex()

		if expectedParentHash != actualParentHash {
			return fmt.Errorf("parent block hash mismatch: expected %s, got %s", expectedParentHash, actualParentHash)
		}

		// Verify block number sequence
		expectedBlockNumber := processingStatus.LastProcessedBlockNumber + 1
		if int64(blockWal.BlockInfo.Number) != expectedBlockNumber {
			return fmt.Errorf("block number sequence error: expected %d, got %d", expectedBlockNumber, blockWal.BlockInfo.Number)
		}
	}

	for _, op := range blockWal.Operations {

		switch {
		case op.Create != nil:
			log.Info("create", "entity", op.Create.EntityKey.Hex())
			err = txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
				Key:                         op.Create.EntityKey.Hex(),
				ExpiresAt:                   int64(op.Create.ExpiresAtBlock),
				Payload:                     op.Create.Payload,
				OwnerAddress:                op.Create.Owner.Hex(),
				CreatedAtBlock:              int64(blockWal.BlockInfo.Number),
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				TransactionIndexInBlock:     int64(op.Create.TransactionIndex),
				OperationIndexInTransaction: int64(op.Create.OperationIndex),
			})
			if err != nil {
				return fmt.Errorf("failed to insert entity: %w", err)
			}

			for _, annotation := range op.Create.NumericAnnotations {
				err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
					EntityKey:                 op.Create.EntityKey.Hex(),
					EntityLastModifiedAtBlock: int64(blockWal.BlockInfo.Number),
					AnnotationKey:             annotation.Key,
					Value:                     int64(annotation.Value),
				})
				if err != nil {
					return fmt.Errorf("failed to insert numeric annotation: %w", err)
				}
			}

			for _, annotation := range op.Create.StringAnnotations {
				err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
					EntityKey:                 op.Create.EntityKey.Hex(),
					EntityLastModifiedAtBlock: int64(blockWal.BlockInfo.Number),
					AnnotationKey:             annotation.Key,
					Value:                     annotation.Value,
				})
				if err != nil {
					return fmt.Errorf("failed to insert string annotation: %w", err)
				}
			}
		case op.Update != nil:
			existingEntity, err := txDB.GetEntity(ctx, sqlitegolem.GetEntityParams{
				Key:   op.Update.EntityKey.Hex(),
				Block: int64(blockWal.BlockInfo.Number - 1),
			})
			if err != nil {
				return fmt.Errorf("failed to get existing entity: %w", err)
			}

			txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
				Key:                         op.Update.EntityKey.Hex(),
				ExpiresAt:                   int64(op.Update.ExpiresAtBlock),
				Payload:                     op.Update.Payload,
				OwnerAddress:                existingEntity.OwnerAddress,
				CreatedAtBlock:              existingEntity.CreatedAtBlock,
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				Deleted:                     false,
				TransactionIndexInBlock:     int64(op.Update.TransactionIndex),
				OperationIndexInTransaction: int64(op.Update.OperationIndex),
			})

			for _, annotation := range op.Update.NumericAnnotations {
				err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
					EntityKey:                 op.Update.EntityKey.Hex(),
					EntityLastModifiedAtBlock: int64(blockWal.BlockInfo.Number),
					AnnotationKey:             annotation.Key,
					Value:                     int64(annotation.Value),
				})
				if err != nil {
					return fmt.Errorf("failed to insert numeric annotation: %w", err)
				}
			}

			for _, annotation := range op.Update.StringAnnotations {
				err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
					EntityKey:                 op.Update.EntityKey.Hex(),
					EntityLastModifiedAtBlock: int64(blockWal.BlockInfo.Number),
					AnnotationKey:             annotation.Key,
					Value:                     annotation.Value,
				})
				if err != nil {
					return fmt.Errorf("failed to insert string annotation: %w", err)
				}
			}
		case op.Delete != nil:
			params := sqlitegolem.DeleteEntityParams{
				Key:                         op.Delete.EntityKey.Hex(),
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				TransactionIndexInBlock:     int64(op.Delete.TransactionIndex),
				OperationIndexInTransaction: int64(op.Delete.OperationIndex),
			}

			log.Info("delete entity", "params", params)

			err = txDB.DeleteEntity(ctx, params)
			if err != nil {
				return fmt.Errorf("failed to delete entity: %w", err)
			}

		case op.Extend != nil:
			extendParams := sqlitegolem.UpdateEntityExpiresAtParams{
				Key:                         op.Extend.EntityKey.Hex(),
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				TransactionIndexInBlock:     int64(op.Extend.TransactionIndex),
				OperationIndexInTransaction: int64(op.Extend.OperationIndex),
				ExpiresAt:                   int64(op.Extend.NewExpiresAt),
			}

			log.Info("extend BTL", "params", extendParams)

			// Update the entity with the new expiry time
			err = txDB.UpdateEntityExpiresAt(ctx, extendParams)
			if err != nil {
				return fmt.Errorf("failed to extend entity BTL: %w", err)
			}
		}

		log.Info("operation", "operation", op)
	}

	err = txDB.UpdateProcessingStatus(ctx, sqlitegolem.UpdateProcessingStatusParams{
		Network:                  networkID,
		LastProcessedBlockNumber: int64(blockWal.BlockInfo.Number),
		LastProcessedBlockHash:   blockWal.BlockInfo.Hash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("failed to insert processing status: %w", err)
	}

	// Delete blocks that are older than the historicBlocksCount
	if e.historicBlocksCount > 0 && blockWal.BlockInfo.Number > e.historicBlocksCount {
		deleteUntilBlock := int64(blockWal.BlockInfo.Number) - int64(e.historicBlocksCount)
		txDB.DeleteStringAnnotationsUntilBlock(ctx, deleteUntilBlock)
		txDB.DeleteNumericAnnotationsUntilBlock(ctx, deleteUntilBlock)
		txDB.DeleteEntitiesUntilBlock(ctx, deleteUntilBlock)
	}

	return tx.Commit()
}

func (e *SQLStore) QueryEntities(
	ctx context.Context,
	query string,
	args []any,
	options query.QueryOptions,
) ([]arkivtype.EntityData, error) {
	log.Info("Executing query", "query", query, "args", args)

	// Begin a read-only transaction for consistency
	tx, err := e.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Safe to call even after commit

	txDB := sqlitegolem.New(tx)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get entities for query: %s: %w", query, err)
	}
	defer rows.Close()

	results := make([]arkivtype.EntityData, 0)
	for rows.Next() {

		result := struct {
			key       *string
			expiresAt *uint64
			payload   *[]byte
			owner     *string
		}{}
		dest := []any{}
		for _, column := range options.Columns {
			switch column {
			case "key":
				var key string
				result.key = &key
				dest = append(dest, result.key)
			case "expires_at":
				var expiration uint64
				result.expiresAt = &expiration
				dest = append(dest, result.expiresAt)
			case "payload":
				var payload []byte
				result.payload = &payload
				dest = append(dest, result.payload)
			case "owner_address":
				var owner string
				result.owner = &owner
				dest = append(dest, result.owner)
			}
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("failed to get entities for query: %s: %w", query, err)
		}

		key := common.Hash{}
		if result.key != nil {
			key = common.HexToHash(*result.key)
		}
		expiresAt := uint64(0)
		if result.expiresAt != nil {
			expiresAt = *result.expiresAt
		}
		var payload []byte = nil
		if result.payload != nil {
			payload = *result.payload
		}
		owner := common.Address{}
		if result.owner != nil {
			owner = common.HexToAddress(*result.owner)
		}

		r := arkivtype.EntityData{
			Key:                key,
			ExpiresAt:          expiresAt,
			Value:              payload,
			Owner:              owner,
			StringAnnotations:  []entity.StringAnnotation{},
			NumericAnnotations: []entity.NumericAnnotation{},
		}

		if options.IncludeAnnotations {
			// Get string annotations
			stringAnnotRows, err := txDB.GetStringAnnotations(ctx, sqlitegolem.GetStringAnnotationsParams{
				EntityKey: key.Hex(),
				Block:     int64(options.AtBlock),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get string annotations: %w", err)
			}

			// Get numeric annotations
			numericAnnotRows, err := txDB.GetNumericAnnotations(ctx, sqlitegolem.GetNumericAnnotationsParams{
				EntityKey: key.Hex(),
				Block:     int64(options.AtBlock),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get numeric annotations: %w", err)
			}

			// Convert string annotations
			for _, row := range stringAnnotRows {
				r.StringAnnotations = append(r.StringAnnotations, entity.StringAnnotation{
					Key:   row.AnnotationKey,
					Value: row.Value,
				})
			}

			// Convert numeric annotations
			for _, row := range numericAnnotRows {
				r.NumericAnnotations = append(r.NumericAnnotations, entity.NumericAnnotation{
					Key:   row.AnnotationKey,
					Value: uint64(row.Value),
				})
			}
		}

		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to get entities for query: %s: %w", query, err)
	}

	// Commit the transaction (read-only, but ensures consistency)
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return results, nil
}
