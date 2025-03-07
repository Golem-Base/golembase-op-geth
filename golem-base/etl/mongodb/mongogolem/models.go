package mongogolem

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProcessingStatus tracks the last processed block
type ProcessingStatus struct {
	Network                  string    `bson:"network"`
	LastProcessedBlockNumber int64     `bson:"last_processed_block_number"`
	LastProcessedBlockHash   string    `bson:"last_processed_block_hash"`
	UpdatedAt                time.Time `bson:"updated_at"`
}

// Entity represents a stored entity
type Entity struct {
	Key       string    `bson:"_id"`
	ExpiresAt int64     `bson:"expires_at"`
	Payload   []byte    `bson:"payload"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// StringAnnotation represents a string annotation for an entity
type StringAnnotation struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	EntityKey     string             `bson:"entity_key"`
	AnnotationKey string             `bson:"annotation_key"`
	Value         string             `bson:"value"`
}

// NumericAnnotation represents a numeric annotation for an entity
type NumericAnnotation struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	EntityKey     string             `bson:"entity_key"`
	AnnotationKey string             `bson:"annotation_key"`
	Value         int64              `bson:"value"`
}
