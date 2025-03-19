package entity

import (
	"github.com/ethereum/go-ethereum/cmd/golembase/entity/create"
	"github.com/urfave/cli/v2"
)

func Entity() *cli.Command {
	return &cli.Command{
		Name:  "entity",
		Usage: "Manage entities",
		Subcommands: []*cli.Command{
			create.Create(),
		},
	}
}
