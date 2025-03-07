package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/ethereum/go-ethereum/golem-base/etl/mongodb/mongogolem"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoTestContext struct {
	container *mongodb.MongoDBContainer
	client    *mongo.Client
	db        *mongo.Database
	driver    *mongogolem.MongoGolem
}

func TestMongoDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	opts := godog.Options{
		NoColors: true,
		Format:   "pretty",
		Paths:    []string{"../../features/mongodb"},
		TestingT: t,
	}

	suite := godog.TestSuite{
		Name: "MongoDB ETL Test",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			initializeMongoDBScenario(sc, t)
		},
		Options: &opts,
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run MongoDB ETL tests")
	}
}

func initializeMongoDBScenario(sc *godog.ScenarioContext, t *testing.T) {
	var ctx context.Context
	var cancel context.CancelFunc
	var mongoCtx mongoTestContext

	// Set up hooks to start and stop the MongoDB container
	sc.BeforeScenario(func(s *godog.Scenario) {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)

		// Start MongoDB container with proper options
		mongoContainer, err := mongodb.RunContainer(ctx,
			testcontainers.WithImage("mongo:6.0"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("Waiting for connections").WithOccurrence(1).WithStartupTimeout(60*time.Second),
			),
		)
		require.NoError(t, err, "Failed to start MongoDB container")

		// Connect to MongoDB
		mongoURI, err := mongoContainer.ConnectionString(ctx)
		require.NoError(t, err, "Failed to get MongoDB connection string")

		// Set MongoDB URI as environment variable for the ETL process
		os.Setenv("MONGO_URI", mongoURI)
		os.Setenv("DB_NAME", "golem_test")

		// Create MongoDB client
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
		require.NoError(t, err, "Failed to connect to MongoDB")

		// Create database and driver
		db := client.Database("golem_test")
		driver := mongogolem.New(db)

		// Create indexes
		err = driver.EnsureIndexes(ctx)
		require.NoError(t, err, "Failed to create indexes")

		// Store context for use in steps
		mongoCtx = mongoTestContext{
			container: mongoContainer,
			client:    client,
			db:        db,
			driver:    driver,
		}
	})

	sc.AfterScenario(func(s *godog.Scenario, err error) {
		// Clean up MongoDB resources
		if mongoCtx.client != nil {
			if err := mongoCtx.client.Disconnect(ctx); err != nil {
				fmt.Printf("Failed to disconnect MongoDB client: %v\n", err)
			}
		}

		if mongoCtx.container != nil {
			if err := mongoCtx.container.Terminate(ctx); err != nil {
				fmt.Printf("Failed to terminate MongoDB container: %v\n", err)
			}
		}

		cancel()
	})

	// Define step definitions for MongoDB-specific tests
	sc.Step(`^I have a MongoDB database$`, func() error {
		// Verify connection is working by pinging the database
		return mongoCtx.client.Ping(ctx, nil)
	})

	sc.Step(`^I insert a test entity$`, func() error {
		entity := mongogolem.Entity{
			Key:       "test_entity",
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
			Payload:   []byte("test payload"),
		}
		return mongoCtx.driver.InsertEntity(ctx, entity)
	})

	sc.Step(`^I can retrieve the test entity$`, func() error {
		cols := mongoCtx.driver.Collections()
		var entity mongogolem.Entity
		err := cols.Entities.FindOne(ctx, bson.M{"_id": "test_entity"}).Decode(&entity)
		if err != nil {
			return fmt.Errorf("failed to find entity: %w", err)
		}
		if string(entity.Payload) != "test payload" {
			return fmt.Errorf("unexpected payload: %s", string(entity.Payload))
		}
		return nil
	})

	sc.Step(`^I insert a test annotation$`, func() error {
		annotation := mongogolem.StringAnnotation{
			EntityKey:     "test_entity",
			AnnotationKey: "test_key",
			Value:         "test_value",
		}
		return mongoCtx.driver.InsertStringAnnotation(ctx, annotation)
	})

	sc.Step(`^I can retrieve the test annotation$`, func() error {
		cols := mongoCtx.driver.Collections()
		var annotation mongogolem.StringAnnotation
		err := cols.StringAnnotations.FindOne(ctx, bson.M{
			"entity_key":     "test_entity",
			"annotation_key": "test_key",
		}).Decode(&annotation)
		if err != nil {
			return fmt.Errorf("failed to find annotation: %w", err)
		}
		if annotation.Value != "test_value" {
			return fmt.Errorf("unexpected value: %s", annotation.Value)
		}
		return nil
	})

	sc.Step(`^I delete the test entity$`, func() error {
		return mongoCtx.driver.DeleteEntity(ctx, "test_entity")
	})

	sc.Step(`^the test entity should be gone$`, func() error {
		cols := mongoCtx.driver.Collections()
		var entity mongogolem.Entity
		err := cols.Entities.FindOne(ctx, bson.M{"_id": "test_entity"}).Decode(&entity)
		if err == nil {
			return fmt.Errorf("entity still exists")
		}
		return nil
	})
}
