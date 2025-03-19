package query

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

func Query() *cli.Command {
	cfg := struct {
		NodeURL string
	}{}
	return &cli.Command{
		Name:  "query",
		Usage: "query entity",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.NodeURL,
			},
		},
		Action: func(c *cli.Context) error {

			ctx, stop := signal.NotifyContext(c.Context, os.Interrupt)
			defer stop()

			query := c.Args().First()
			if query == "" {
				return fmt.Errorf("query is required")
			}
			// Connect to the geth node
			rpcClient, err := rpc.Dial(cfg.NodeURL)
			if err != nil {
				return fmt.Errorf("failed to connect to node: %w", err)
			}
			defer rpcClient.Close()

			res := []golemtype.SearchResult{}

			err = rpcClient.CallContext(
				ctx,
				&res,
				"golembase_queryEntities",
				query,
			)
			if err != nil {
				return fmt.Errorf("failed to get entities to by numeric annotation: %w", err)
			}

			for _, r := range res {
				fmt.Println(r.Key)
				fmt.Println("  payload:", string(r.Value))
			}

			return nil
		},
	}
}

func pointerOf[T any](v T) *T {
	return &v
}
