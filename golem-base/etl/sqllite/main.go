package main

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

//go:generate sqlc generate

//go:embed schema.sql
var schema string

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := struct {
		dbFile      string
		walDir      string
		rpcEndpoint string
	}{}
	app := &cli.App{
		Name: "sqllite-etl",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "db",
				Usage:       "database file",
				EnvVars:     []string{"DB_FILE"},
				Destination: &cfg.dbFile,
				Required:    true,
			},
			&cli.PathFlag{
				Name:        "wal",
				Usage:       "wal dir",
				EnvVars:     []string{"WAL_DIR"},
				Required:    true,
				Destination: &cfg.walDir,
			},
			&cli.StringFlag{
				Name:        "rpc-endpoint",
				Usage:       "RPC Endpoint for op-geth",
				EnvVars:     []string{"RPC_ENDPOINT"},
				Required:    true,
				Destination: &cfg.rpcEndpoint,
			},
		},
		Action: func(c *cli.Context) error {

			ctx, cancel := signal.NotifyContext(c.Context, os.Interrupt)
			defer cancel()

			db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", cfg.dbFile))
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()

			var tableName string
			err = db.QueryRowContext(ctx, `
				SELECT name FROM sqlite_master 
				WHERE type='table' AND name='entities';
			`).Scan(&tableName)

			if err == sql.ErrNoRows {
				log.Info("could not find 'entities' table, applying schema")
				_, err := db.ExecContext(ctx, schema)
				if err != nil {
					return fmt.Errorf("failed to apply schema table: %w", err)
				}
			}

			autocommit := New(db)

			ec, err := ethclient.Dial(cfg.rpcEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial rpc endpoint: %w", err)
			}

			networkID, err := ec.NetworkID(ctx)
			if err != nil {
				return fmt.Errorf("failed to get network id: %w", err)
			}

			processingStatus, err := autocommit.HasProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to check if processing status exists: %w", err)
			}

			if !processingStatus {
				log.Info("no processing status found, inserting genesis block")

				genesisHeade, err := ec.HeaderByNumber(ctx, big.NewInt(0))
				if err != nil {
					return fmt.Errorf("failed to get genesis header: %w", err)
				}

				err = autocommit.InsertProcessingStatus(ctx, InsertProcessingStatusParams{
					Network:                  networkID.String(),
					LastProcessedBlockNumber: 0,
					LastProcessedBlockHash:   genesisHeade.Hash().String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert processing status: %w", err)
				}
			}

			// for blockWal, err := range wal.NewIterator(ctx, cfg.walDir, 0, common.Hash{}, false) {
			// }

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error("failed to run app", "error", err)
		os.Exit(1)
	}
}
