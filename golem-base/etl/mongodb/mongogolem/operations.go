package mongogolem

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// HasProcessingStatus checks if a processing status exists for the given network
func (m *MongoGolem) HasProcessingStatus(ctx context.Context, network string) (bool, error) {
	cols := m.Collections()

	count, err := cols.ProcessingStatus.CountDocuments(ctx, bson.M{"network": network})
	if err != nil {
		return false, fmt.Errorf("failed to count processing status: %w", err)
	}

	return count > 0, nil
}

// GetProcessingStatus retrieves the processing status for the given network
func (m *MongoGolem) GetProcessingStatus(ctx context.Context, network string) (ProcessingStatus, error) {
	cols := m.Collections()

	var status ProcessingStatus
	err := cols.ProcessingStatus.FindOne(ctx, bson.M{"network": network}).Decode(&status)
	if err != nil {
		return ProcessingStatus{}, fmt.Errorf("failed to get processing status: %w", err)
	}

	return status, nil
}

// InsertProcessingStatus inserts a new processing status
func (m *MongoGolem) InsertProcessingStatus(ctx context.Context, params ProcessingStatus) error {
	cols := m.Collections()

	params.UpdatedAt = time.Now()

	_, err := cols.ProcessingStatus.InsertOne(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert processing status: %w", err)
	}

	return nil
}

// UpdateProcessingStatus updates the processing status
func (m *MongoGolem) UpdateProcessingStatus(ctx context.Context, params ProcessingStatus) error {
	cols := m.Collections()

	params.UpdatedAt = time.Now()

	_, err := cols.ProcessingStatus.UpdateOne(
		ctx,
		bson.M{"network": params.Network},
		bson.M{"$set": params},
	)
	if err != nil {
		return fmt.Errorf("failed to update processing status: %w", err)
	}

	return nil
}

// InsertEntity inserts a new entity
func (m *MongoGolem) InsertEntity(ctx context.Context, params Entity) error {
	cols := m.Collections()

	now := time.Now()
	params.CreatedAt = now
	params.UpdatedAt = now

	_, err := cols.Entities.InsertOne(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert entity: %w", err)
	}

	return nil
}

// UpdateEntity updates an existing entity
func (m *MongoGolem) UpdateEntity(ctx context.Context, params Entity) error {
	cols := m.Collections()

	params.UpdatedAt = time.Now()

	_, err := cols.Entities.ReplaceOne(
		ctx,
		bson.M{"_id": params.Key},
		params,
	)
	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	return nil
}

// DeleteEntity deletes an entity by key
func (m *MongoGolem) DeleteEntity(ctx context.Context, key string) error {
	cols := m.Collections()

	_, err := cols.Entities.DeleteOne(ctx, bson.M{"_id": key})
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	return nil
}

// InsertStringAnnotation inserts a string annotation
func (m *MongoGolem) InsertStringAnnotation(ctx context.Context, params StringAnnotation) error {
	cols := m.Collections()

	// Use upsert to handle potential duplicates
	opts := options.Update().SetUpsert(true)

	_, err := cols.StringAnnotations.UpdateOne(
		ctx,
		bson.M{
			"entity_key":     params.EntityKey,
			"annotation_key": params.AnnotationKey,
		},
		bson.M{"$set": params},
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to insert string annotation: %w", err)
	}

	return nil
}

// InsertNumericAnnotation inserts a numeric annotation
func (m *MongoGolem) InsertNumericAnnotation(ctx context.Context, params NumericAnnotation) error {
	cols := m.Collections()

	// Use upsert to handle potential duplicates
	opts := options.Update().SetUpsert(true)

	_, err := cols.NumericAnnotations.UpdateOne(
		ctx,
		bson.M{
			"entity_key":     params.EntityKey,
			"annotation_key": params.AnnotationKey,
		},
		bson.M{"$set": params},
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to insert numeric annotation: %w", err)
	}

	return nil
}

// DeleteStringAnnotations deletes all string annotations for an entity
func (m *MongoGolem) DeleteStringAnnotations(ctx context.Context, entityKey string) error {
	cols := m.Collections()

	_, err := cols.StringAnnotations.DeleteMany(ctx, bson.M{"entity_key": entityKey})
	if err != nil {
		return fmt.Errorf("failed to delete string annotations: %w", err)
	}

	return nil
}

// DeleteNumericAnnotations deletes all numeric annotations for an entity
func (m *MongoGolem) DeleteNumericAnnotations(ctx context.Context, entityKey string) error {
	cols := m.Collections()

	_, err := cols.NumericAnnotations.DeleteMany(ctx, bson.M{"entity_key": entityKey})
	if err != nil {
		return fmt.Errorf("failed to delete numeric annotations: %w", err)
	}

	return nil
}
