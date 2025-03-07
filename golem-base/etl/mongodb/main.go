package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/golem-base/etl/mongodb/mongogolem"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := struct {
		mongoURI    string
		dbName      string
		walDir      string
		rpcEndpoint string
	}{}

	app := &cli.App{
		Name: "mongodb-etl",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "mongo-uri",
				Usage:       "MongoDB connection URI",
				EnvVars:     []string{"MONGO_URI"},
				Destination: &cfg.mongoURI,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "db-name",
				Usage:       "MongoDB database name",
				EnvVars:     []string{"DB_NAME"},
				Destination: &cfg.dbName,
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

			// Connect to MongoDB
			clientOptions := options.Client().ApplyURI(cfg.mongoURI)
			client, err := mongo.Connect(ctx, clientOptions)
			if err != nil {
				return fmt.Errorf("failed to connect to MongoDB: %w", err)
			}
			defer func() {
				if err := client.Disconnect(ctx); err != nil {
					log.Error("failed to disconnect from MongoDB", "error", err)
				}
			}()

			// Ping the MongoDB server to ensure connection is established
			if err := client.Ping(ctx, nil); err != nil {
				return fmt.Errorf("failed to ping MongoDB: %w", err)
			}
			log.Info("Connected to MongoDB")

			// Get database and create MongoDB driver
			db := client.Database(cfg.dbName)
			mongoDriver := mongogolem.New(db)

			// Create indexes
			if err := mongoDriver.EnsureIndexes(ctx); err != nil {
				return fmt.Errorf("failed to ensure indexes: %w", err)
			}

			ec, err := ethclient.Dial(cfg.rpcEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial rpc endpoint: %w", err)
			}

			networkID, err := ec.NetworkID(ctx)
			if err != nil {
				return fmt.Errorf("failed to get network id: %w", err)
			}

			hasProcessingStatus, err := mongoDriver.HasProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to check if processing status exists: %w", err)
			}

			if !hasProcessingStatus {
				log.Info("no processing status found, inserting genesis block")

				genesisHeader, err := ec.HeaderByNumber(ctx, big.NewInt(0))
				if err != nil {
					return fmt.Errorf("failed to get genesis header: %w", err)
				}

				err = mongoDriver.InsertProcessingStatus(ctx, mongogolem.ProcessingStatus{
					Network:                  networkID.String(),
					LastProcessedBlockNumber: 0,
					LastProcessedBlockHash:   genesisHeader.Hash().String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert processing status: %w", err)
				}
			}

			processingStatus, err := mongoDriver.GetProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to get processing status: %w", err)
			}

			blockNumber := processingStatus.LastProcessedBlockNumber
			blockHash := processingStatus.LastProcessedBlockHash

			for blockWal, err := range wal.NewIterator(ctx, cfg.walDir, uint64(blockNumber)+1, common.HexToHash(blockHash), true) {
				if err != nil {
					return fmt.Errorf("failed to iterate over wal: %w", err)
				}

				err = func() (err error) {
					log.Info("processing block", "block", blockWal.BlockInfo.Number)

					// Create a session with a timeout for each block processing
					sessCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					session, err := client.StartSession()
					if err != nil {
						return fmt.Errorf("failed to start MongoDB session: %w", err)
					}
					defer session.EndSession(sessCtx)

					// Use WithTransaction to handle transactions
					_, err = session.WithTransaction(sessCtx, func(txCtx mongo.SessionContext) (interface{}, error) {
						for op, err := range blockWal.OperationsIterator {
							if err != nil {
								return nil, fmt.Errorf("failed to iterate over operations: %w", err)
							}

							switch {
							case op.Create != nil:
								log.Info("create", "entity", op.Create.EntityKey.Hex())
								err = mongoDriver.InsertEntity(txCtx, mongogolem.Entity{
									Key:       op.Create.EntityKey.Hex(),
									ExpiresAt: int64(op.Create.ExpiresAtBlock),
									Payload:   op.Create.Payload,
								})
								if err != nil {
									return nil, fmt.Errorf("failed to insert entity: %w", err)
								}

								for _, annotation := range op.Create.NumericAnnotations {
									err = mongoDriver.InsertNumericAnnotation(txCtx, mongogolem.NumericAnnotation{
										EntityKey:     op.Create.EntityKey.Hex(),
										AnnotationKey: annotation.Key,
										Value:         int64(annotation.Value),
									})
									if err != nil {
										return nil, fmt.Errorf("failed to insert numeric annotation: %w", err)
									}
								}

								for _, annotation := range op.Create.StringAnnotations {
									err = mongoDriver.InsertStringAnnotation(txCtx, mongogolem.StringAnnotation{
										EntityKey:     op.Create.EntityKey.Hex(),
										AnnotationKey: annotation.Key,
										Value:         annotation.Value,
									})
									if err != nil {
										return nil, fmt.Errorf("failed to insert string annotation: %w", err)
									}
								}

							case op.Update != nil:
								// First delete existing data
								err = mongoDriver.DeleteEntity(txCtx, op.Update.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete entity before update: %w", err)
								}

								err = mongoDriver.DeleteNumericAnnotations(txCtx, op.Update.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete numeric annotations before update: %w", err)
								}

								err = mongoDriver.DeleteStringAnnotations(txCtx, op.Update.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete string annotations before update: %w", err)
								}

								// Then insert updated data
								err = mongoDriver.InsertEntity(txCtx, mongogolem.Entity{
									Key:       op.Update.EntityKey.Hex(),
									ExpiresAt: int64(op.Update.ExpiresAtBlock),
									Payload:   op.Update.Payload,
								})
								if err != nil {
									return nil, fmt.Errorf("failed to insert updated entity: %w", err)
								}

								for _, annotation := range op.Update.NumericAnnotations {
									err = mongoDriver.InsertNumericAnnotation(txCtx, mongogolem.NumericAnnotation{
										EntityKey:     op.Update.EntityKey.Hex(),
										AnnotationKey: annotation.Key,
										Value:         int64(annotation.Value),
									})
									if err != nil {
										return nil, fmt.Errorf("failed to insert updated numeric annotation: %w", err)
									}
								}

								for _, annotation := range op.Update.StringAnnotations {
									err = mongoDriver.InsertStringAnnotation(txCtx, mongogolem.StringAnnotation{
										EntityKey:     op.Update.EntityKey.Hex(),
										AnnotationKey: annotation.Key,
										Value:         annotation.Value,
									})
									if err != nil {
										return nil, fmt.Errorf("failed to insert updated string annotation: %w", err)
									}
								}

							case op.Delete != nil:
								err = mongoDriver.DeleteEntity(txCtx, op.Delete.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete entity: %w", err)
								}

								err = mongoDriver.DeleteNumericAnnotations(txCtx, op.Delete.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete numeric annotations: %w", err)
								}

								err = mongoDriver.DeleteStringAnnotations(txCtx, op.Delete.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete string annotations: %w", err)
								}
							}

							log.Info("operation", "operation", op)
						}

						// Update processing status
						err = mongoDriver.UpdateProcessingStatus(txCtx, mongogolem.ProcessingStatus{
							Network:                  networkID.String(),
							LastProcessedBlockNumber: int64(blockWal.BlockInfo.Number),
							LastProcessedBlockHash:   blockWal.BlockInfo.Hash.String(),
						})
						if err != nil {
							return nil, fmt.Errorf("failed to update processing status: %w", err)
						}

						return nil, nil
					})

					return err
				}()

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
