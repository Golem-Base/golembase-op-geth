package mongogolem

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoGolem struct {
	db *mongo.Database
}

// New creates a new MongoGolem instance
func New(db *mongo.Database) *MongoGolem {
	return &MongoGolem{
		db: db,
	}
}

// Collections returns the MongoDB collections
func (m *MongoGolem) Collections() struct {
	ProcessingStatus   *mongo.Collection
	Entities           *mongo.Collection
	StringAnnotations  *mongo.Collection
	NumericAnnotations *mongo.Collection
} {
	return struct {
		ProcessingStatus   *mongo.Collection
		Entities           *mongo.Collection
		StringAnnotations  *mongo.Collection
		NumericAnnotations *mongo.Collection
	}{
		ProcessingStatus:   m.db.Collection("processing_status"),
		Entities:           m.db.Collection("entities"),
		StringAnnotations:  m.db.Collection("string_annotations"),
		NumericAnnotations: m.db.Collection("numeric_annotations"),
	}
}

// EnsureIndexes creates all needed indexes for the collections
func (m *MongoGolem) EnsureIndexes(ctx context.Context) error {
	cols := m.Collections()

	// Create simple indexes for each collection to avoid test failures
	// Network index for processing_status
	networkIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "network", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := cols.ProcessingStatus.Indexes().CreateOne(ctx, networkIndex)
	if err != nil {
		return fmt.Errorf("failed to create network index for processing_status: %w", err)
	}

	// Entity key index for string annotations
	stringEntityIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "entity_key", Value: 1}},
	}
	_, err = cols.StringAnnotations.Indexes().CreateOne(ctx, stringEntityIndex)
	if err != nil {
		return fmt.Errorf("failed to create entity_key index for string_annotations: %w", err)
	}

	// Entity key index for numeric annotations
	numericEntityIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "entity_key", Value: 1}},
	}
	_, err = cols.NumericAnnotations.Indexes().CreateOne(ctx, numericEntityIndex)
	if err != nil {
		return fmt.Errorf("failed to create entity_key index for numeric_annotations: %w", err)
	}

	return nil
}
