package main_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqllite/etlworld"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqllite/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag" // godog v0.11.0 and later
)

var opts = godog.Options{
	Output:      colors.Uncolored(os.Stdout),
	Format:      "progress",
	Strict:      true,
	Concurrency: 4,

	Paths: []string{"features"},
}

func init() {
	godog.BindCommandLineFlags("godog.", &opts)

	if os.Getenv("CUCUMBER_WIP_ONLY") == "true" {
		opts.Tags = "@wip"
		opts.Concurrency = 1
		opts.Format = "pretty"
	}
}

func compileGeth() (string, func(), error) {
	td, err := os.MkdirTemp("", "golem-base")
	if err != nil {
		panic(fmt.Errorf("failed to create temp dir: %w", err))
	}

	gethBinaryPath := filepath.Join(td, "geth")

	cmd := exec.Command("go", "build", "-o", gethBinaryPath, "../../../cmd/geth")
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to compile geth: %w\n%s", err, out.String())
	}

	return gethBinaryPath, func() {
		os.RemoveAll(td)
	}, nil
}

func compileSqlliteETL() (string, func(), error) {
	td, err := os.MkdirTemp("", "sqllite-etl")
	if err != nil {
		panic(fmt.Errorf("failed to create temp dir: %w", err))
	}

	sqlliteETLBinaryPath := filepath.Join(td, "sqllite-etl")

	cmd := exec.Command("go", "build", "-o", sqlliteETLBinaryPath, ".")
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to compile sqllite-etl: %w\n%s", err, out.String())
	}

	return sqlliteETLBinaryPath, func() {
		os.RemoveAll(td)
	}, nil
}

func TestMain(m *testing.M) {
	pflag.Parse()
	opts.Paths = pflag.Args()

	gethPath, cleanupCompiled, err := compileGeth()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to compile geth: %w", err))
	}

	sqlliteETLPath, cleanupCompiledSQLLiteETL, err := compileSqlliteETL()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to compile geth: %w", err))
	}

	suite := godog.TestSuite{
		Name: "cucumber",
		ScenarioInitializer: func(sctx *godog.ScenarioContext) {
			InitializeScenario(sctx)
			sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {

				world, err := etlworld.NewETLWorld(ctx, gethPath, sqlliteETLPath)
				if err != nil {
					return ctx, fmt.Errorf("failed to start geth instance: %w", err)
				}

				timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

				sctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
					world.Shutdown()
					cancel()
					return ctx, world.AddLogsToTestError(err)
				})

				return etlworld.WithWorld(timeoutCtx, world), nil

			})

		},
		// ScenarioInitializer:  InitializeScenario,
		Options: &opts,
	}

	status := suite.Run()

	// // Optional: Run `testing` package's logic besides godog.
	// if st := m.Run(); st > status {
	// 	status = st
	// }

	cleanupCompiled()
	cleanupCompiledSQLLiteETL()

	os.Exit(status)
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^A running ETL to SQLite$`, aRunningETLToSQLite)
	ctx.Step(`^A running Golembase node with WAL enabled$`, aRunningGolembaseNodeWithWALEnabled)
	ctx.Step(`^I create a new entity in Golebase$`, iCreateANewEntityInGolebase)
	ctx.Step(`^the entity should be created in the SQLite database$`, theEntityShouldBeCreatedInTheSQLiteDatabase)
	ctx.Step(`^the annotations of the entity should be existing in the SQLite database$`, theAnnotationsOfTheEntityShouldBeExistingInTheSQLiteDatabase)
	ctx.Step(`^an existing entity in the SQLite database$`, anExistingEntityInTheSQLiteDatabase)
	ctx.Step(`^the annotations of the entity should be updated in the SQLite database$`, theAnnotationsOfTheEntityShouldBeUpdatedInTheSQLiteDatabase)
	ctx.Step(`^the entity should be updated in the SQLite database$`, theEntityShouldBeUpdatedInTheSQLiteDatabase)
	ctx.Step(`^update the entity in Golembase$`, updateTheEntityInGolembase)

}

func aRunningETLToSQLite() error {
	// this is a default when starting etlworld.World, so we don't need to do anything here
	return nil
}

func aRunningGolembaseNodeWithWALEnabled() error {
	// this is a default when starting testutil.World, so we don't need to do anything here
	return nil
}

func iCreateANewEntityInGolebase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.CreateEntity(ctx,
		1000,
		[]byte("test"),
		[]storageutil.StringAnnotation{
			{
				Key:   "stringTest",
				Value: "stringTest",
			},
		},
		[]storageutil.NumericAnnotation{
			{
				Key:   "numericTest",
				Value: 1234567890,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func theEntityShouldBeCreatedInTheSQLiteDatabase(ctx context.Context) error {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	w := etlworld.GetWorld(ctx)

	bo := backoff.WithContext(backoff.NewConstantBackOff(100*time.Millisecond), ctx)

	err := backoff.Retry(func() error {
		err := w.WithDB(ctx, func(db *sql.DB) error {
			gl := sqlitegolem.New(db)
			entity, err := gl.GetEntity(ctx, w.CreatedEntityKey.Hex())
			if err != nil {
				return fmt.Errorf("failed to get entity: %w", err)
			}

			if entity.Payload == nil {
				return fmt.Errorf("entity payload is nil")
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to check entity in database: %w", err)
		}
		return nil
	}, bo)
	if err != nil {
		return fmt.Errorf("failed to check entity in database: %w", err)
	}

	return nil
}

func theAnnotationsOfTheEntityShouldBeExistingInTheSQLiteDatabase(ctx context.Context) error {

	w := etlworld.GetWorld(ctx)

	var numericAnnotations []sqlitegolem.GetNumericAnnotationsRow
	var stringAnnotations []sqlitegolem.GetStringAnnotationsRow

	err := w.WithDB(ctx, func(db *sql.DB) (err error) {
		gl := sqlitegolem.New(db)
		numericAnnotations, err = gl.GetNumericAnnotations(ctx, w.CreatedEntityKey.Hex())
		if err != nil {
			return fmt.Errorf("failed to get entity: %w", err)
		}

		stringAnnotations, err = gl.GetStringAnnotations(ctx, w.CreatedEntityKey.Hex())
		if err != nil {
			return fmt.Errorf("failed to get entity: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to check entity in database: %w", err)
	}

	expectedStringAnnotations := []sqlitegolem.GetStringAnnotationsRow{
		{
			AnnotationKey: "stringTest",
			Value:         "stringTest",
		},
	}

	if diff := cmp.Diff(stringAnnotations, expectedStringAnnotations); diff != "" {
		return fmt.Errorf("string annotations are not equal: %s", diff)
	}

	expectedNumericAnnotations := []sqlitegolem.GetNumericAnnotationsRow{
		{
			AnnotationKey: "numericTest",
			Value:         1234567890,
		},
	}

	if diff := cmp.Diff(numericAnnotations, expectedNumericAnnotations); diff != "" {
		return fmt.Errorf("numeric annotations are not equal: %s", diff)
	}

	return nil
}

func anExistingEntityInTheSQLiteDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.CreateEntity(ctx,
		1000,
		[]byte("test"),
		[]storageutil.StringAnnotation{
			{
				Key:   "stringTest",
				Value: "stringTest",
			},
		},
		[]storageutil.NumericAnnotation{
			{
				Key:   "numericTest",
				Value: 1234567890,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func theAnnotationsOfTheEntityShouldBeUpdatedInTheSQLiteDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)

	var numericAnnotations []sqlitegolem.GetNumericAnnotationsRow
	var stringAnnotations []sqlitegolem.GetStringAnnotationsRow

	err := w.WithDB(ctx, func(db *sql.DB) (err error) {
		gl := sqlitegolem.New(db)
		numericAnnotations, err = gl.GetNumericAnnotations(ctx, w.CreatedEntityKey.Hex())
		if err != nil {
			return fmt.Errorf("failed to get entity: %w", err)
		}

		stringAnnotations, err = gl.GetStringAnnotations(ctx, w.CreatedEntityKey.Hex())
		if err != nil {
			return fmt.Errorf("failed to get entity: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to check entity in database: %w", err)
	}

	expectedStringAnnotations := []sqlitegolem.GetStringAnnotationsRow{
		{
			AnnotationKey: "stringTest2",
			Value:         "stringTest2",
		},
	}

	if diff := cmp.Diff(stringAnnotations, expectedStringAnnotations); diff != "" {
		return fmt.Errorf("string annotations are not equal: %s", diff)
	}

	expectedNumericAnnotations := []sqlitegolem.GetNumericAnnotationsRow{
		{
			AnnotationKey: "numericTest2",
			Value:         12345678901,
		},
	}

	if diff := cmp.Diff(numericAnnotations, expectedNumericAnnotations); diff != "" {
		return fmt.Errorf("numeric annotations are not equal: %s", diff)
	}

	return nil

}

func theEntityShouldBeUpdatedInTheSQLiteDatabase(ctx context.Context) error {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	w := etlworld.GetWorld(ctx)

	bo := backoff.WithContext(backoff.NewConstantBackOff(100*time.Millisecond), ctx)

	err := backoff.Retry(func() error {
		err := w.WithDB(ctx, func(db *sql.DB) error {
			gl := sqlitegolem.New(db)
			entity, err := gl.GetEntity(ctx, w.CreatedEntityKey.Hex())
			if err != nil {
				return fmt.Errorf("failed to get entity: %w", err)
			}

			if entity.Payload == nil {
				return fmt.Errorf("entity payload is nil")
			}

			if string(entity.Payload) != "test2" {
				return fmt.Errorf("entity payload is not equal to test2")
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to check entity in database: %w", err)
		}
		return nil
	}, bo)
	if err != nil {
		return fmt.Errorf("failed to check entity in database: %w", err)
	}

	return nil

}

func updateTheEntityInGolembase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.UpdateEntity(ctx,
		w.CreatedEntityKey,
		999,
		[]byte("test2"),
		[]storageutil.StringAnnotation{
			{
				Key:   "stringTest2",
				Value: "stringTest2",
			},
		},
		[]storageutil.NumericAnnotation{
			{
				Key:   "numericTest2",
				Value: 12345678901,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}
	return nil
}
