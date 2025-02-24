package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

//go:generate sqlc generate

func main() {
	app := &cli.App{
		Name: "sqllite-etl",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "db",
				Value: "golem.db",
				Usage: "database file",
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
