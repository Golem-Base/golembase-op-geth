package entitiestoexpireatblock

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

func EntitiesToExpireAtBlock() *cli.Command {
	cfg := struct {
		nodeURL     string
		block       uint64
		blockNumber uint64
	}{}
	return &cli.Command{
		Name:  "entities-to-expire-at-block",
		Usage: "List all entities that will expire at a given block number",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.nodeURL,
			},
			&cli.Uint64Flag{
				Name:        "at-block",
				Usage:       "The block number to list state for",
				Required:    true,
				Value:       0,
				Destination: &cfg.block,
			},
			&cli.Uint64Flag{
				Name:        "block-number",
				Usage:       "The block number to list entities for",
				Required:    true,
				Destination: &cfg.blockNumber,
			},
		},
		Action: func(c *cli.Context) error {

			ctx, stop := signal.NotifyContext(c.Context, os.Interrupt)
			defer stop()

			rpcClient, err := rpc.Dial(cfg.nodeURL)
			if err != nil {
				return fmt.Errorf("failed to connect to node: %w", err)
			}
			defer rpcClient.Close()

			st := newRPCStateAccess(rpcClient, ctx, cfg.block)

			fmt.Println("Entities to expire at block", cfg.blockNumber)
			for entityHash := range entityexpiration.IteratorOfEntitiesToExpireAtBlock(st, cfg.blockNumber) {
				fmt.Println(entityHash)
			}

			return nil
		},
	}
}

func newRPCStateAccess(rpcClient *rpc.Client, ctx context.Context, block uint64) storageutil.StateAccess {
	return &RPCStateAccess{rpcClient, block, ctx}
}

type RPCStateAccess struct {
	rpcClient *rpc.Client
	atBlock   uint64
	ctx       context.Context
}

func (s *RPCStateAccess) GetState(a common.Address, slot common.Hash) common.Hash {
	var res common.Hash
	err := s.rpcClient.CallContext(s.ctx, &res, "eth_getStorageAt", a, slot, hexutil.Uint64(s.atBlock))
	if err != nil {
		panic(err)
	}
	return res
}

func (s *RPCStateAccess) SetState(a common.Address, slot common.Hash, value common.Hash) common.Hash {
	panic("not implemented")
}
