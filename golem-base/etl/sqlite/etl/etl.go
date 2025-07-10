package etl

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ethereum/go-ethereum/golem-base/etl/sqlite/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	_ "github.com/mattn/go-sqlite3"
)

// ETL encapsulates the SQLite ETL functionality
type ETL struct {
	db *sql.DB
}

// NewETL creates a new ETL instance with database connection and schema setup
func NewETL(dbFile string) (*ETL, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", dbFile))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Check if schema exists and apply if needed
	ctx := context.Background()
	var tableName string
	err = db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='entities';
	`).Scan(&tableName)

	switch err {
	case sql.ErrNoRows:
		err = sqlitegolem.ApplySchema(ctx, db)
		if err != nil {
			db.Close()
			return nil, err
		}
	case nil:
		// schema exists, do nothing
	default:
		db.Close()
		return nil, fmt.Errorf("failed to check schema: %w", err)
	}

	return &ETL{db: db}, nil
}

// Close closes the database connection
func (e *ETL) Close() error {
	return e.db.Close()
}

// GetQueries returns a new sqlitegolem.Queries instance for autocommit operations
func (e *ETL) GetQueries() *sqlitegolem.Queries {
	return sqlitegolem.New(e.db)
}

// InsertBlock processes a single block from the WAL and inserts it into the database
func (e *ETL) InsertBlock(ctx context.Context, blockWal wal.BlockWal, networkID string, log *slog.Logger) error {
	log.Info("processing block", "block", blockWal.BlockInfo.Number)

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

	for op, err := range blockWal.OperationsIterator {
		if err != nil {
			return fmt.Errorf("failed to iterate over operations: %w", err)
		}

		switch {
		case op.Create != nil:
			log.Info("create", "entity", op.Create.EntityKey.Hex())
			err = txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
				Key:          op.Create.EntityKey.Hex(),
				ExpiresAt:    int64(op.Create.ExpiresAtBlock),
				Payload:      op.Create.Payload,
				OwnerAddress: op.Create.Owner.Hex(),
			})
			if err != nil {
				return fmt.Errorf("failed to insert entity: %w", err)
			}

			for _, annotation := range op.Create.NumericAnnotations {
				err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
					EntityKey:     op.Create.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         int64(annotation.Value),
				})
				if err != nil {
					return fmt.Errorf("failed to insert numeric annotation: %w", err)
				}
			}

			for _, annotation := range op.Create.StringAnnotations {
				err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
					EntityKey:     op.Create.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         annotation.Value,
				})
				if err != nil {
					return fmt.Errorf("failed to insert string annotation: %w", err)
				}
			}
		case op.Update != nil:
			existingEntity, err := txDB.GetEntity(ctx, op.Update.EntityKey.Hex())
			if err != nil {
				return fmt.Errorf("failed to get existing entity: %w", err)
			}

			txDB.DeleteEntity(ctx, op.Update.EntityKey.Hex())
			txDB.DeleteNumericAnnotations(ctx, op.Update.EntityKey.Hex())
			txDB.DeleteStringAnnotations(ctx, op.Update.EntityKey.Hex())

			txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
				Key:          op.Update.EntityKey.Hex(),
				ExpiresAt:    int64(op.Update.ExpiresAtBlock),
				Payload:      op.Update.Payload,
				OwnerAddress: existingEntity.OwnerAddress,
			})

			for _, annotation := range op.Update.NumericAnnotations {
				err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
					EntityKey:     op.Update.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         int64(annotation.Value),
				})
				if err != nil {
					return fmt.Errorf("failed to insert numeric annotation: %w", err)
				}
			}

			for _, annotation := range op.Update.StringAnnotations {
				err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
					EntityKey:     op.Update.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         annotation.Value,
				})
				if err != nil {
					return fmt.Errorf("failed to insert string annotation: %w", err)
				}
			}
		case op.Delete != nil:
			err = txDB.DeleteEntity(ctx, op.Delete.Hex())
			if err != nil {
				return fmt.Errorf("failed to delete entity: %w", err)
			}

			err = txDB.DeleteNumericAnnotations(ctx, op.Delete.Hex())
			if err != nil {
				return fmt.Errorf("failed to delete numeric annotations: %w", err)
			}

			err = txDB.DeleteStringAnnotations(ctx, op.Delete.Hex())
			if err != nil {
				return fmt.Errorf("failed to delete string annotations: %w", err)
			}
		case op.Extend != nil:
			log.Info("extend BTL", "entity", op.Extend.EntityKey.Hex())

			// Update the entity with the new expiry time
			err = txDB.UpdateEntityExpiresAt(ctx, sqlitegolem.UpdateEntityExpiresAtParams{
				ExpiresAt: int64(op.Extend.NewExpiresAt),
				Key:       op.Extend.EntityKey.Hex(),
			})
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

	return tx.Commit()
}
