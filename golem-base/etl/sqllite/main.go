package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

//go:generate sqlc generate

func main() {
	cfg := struct {
		dbFile string
		walDir string
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
		},
		Action: func(c *cli.Context) error {

			fmt.Println("Hello, World!")
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
