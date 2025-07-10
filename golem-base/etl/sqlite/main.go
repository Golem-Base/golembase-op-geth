package main

import (
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqlite/etl"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqlite/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	"github.com/urfave/cli/v2"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := struct {
		dbFile      string
		walDir      string
		rpcEndpoint string
	}{}
	app := &cli.App{
		Name: "sqlite-etl",
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

			// Create ETL instance
			etlInstance, err := etl.NewETL(cfg.dbFile)
			if err != nil {
				return fmt.Errorf("failed to create ETL instance: %w", err)
			}
			defer etlInstance.Close()

			// Get queries instance for autocommit operations
			autocommit := etlInstance.GetQueries()

			ec, err := ethclient.Dial(cfg.rpcEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial rpc endpoint: %w", err)
			}

			networkID, err := ec.NetworkID(ctx)
			if err != nil {
				return fmt.Errorf("failed to get network id: %w", err)
			}

			hasProcessingStatus, err := autocommit.HasProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to check if processing status exists: %w", err)
			}

			if !hasProcessingStatus {
				log.Info("no processing status found, inserting genesis block")

				genesisHeader, err := ec.HeaderByNumber(ctx, big.NewInt(0))
				if err != nil {
					return fmt.Errorf("failed to get genesis header: %w", err)
				}

				err = autocommit.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
					Network:                  networkID.String(),
					LastProcessedBlockNumber: 0,
					LastProcessedBlockHash:   genesisHeader.Hash().String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert processing status: %w", err)
				}
			}

			processingStatus, err := autocommit.GetProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to get processing status: %w", err)
			}

			blockNumber := processingStatus.LastProcessedBlockNumber
			blockHash := processingStatus.LastProcessedBlockHash

			for blockWal, err := range wal.NewIterator(ctx, cfg.walDir, uint64(blockNumber)+1, common.HexToHash(blockHash), true) {
				if err != nil {
					return fmt.Errorf("failed to iterate over wal: %w", err)
				}

				err = etlInstance.InsertBlock(ctx, blockWal, networkID.String(), log)
				if err != nil {
					return fmt.Errorf("failed to process block: %w", err)
				}
			}

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error("failed to run app", "error", err)
		os.Exit(1)
	}
}
