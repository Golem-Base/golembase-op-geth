package main

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

//go:generate sqlc generate

//go:embed schema.sql
var schema string

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
				_, err := db.ExecContext(ctx, schema)
				if err != nil {
					return fmt.Errorf("failed to apply schema table: %w", err)
				}
			}

			if err != nil {
				return fmt.Errorf("failed to check if table exists: %w", err)
			}

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
