# MongoDB ETL for Golem

This package implements an ETL (Extract, Transform, Load) process for the Golem system using MongoDB as the storage backend.

## Overview

The MongoDB ETL reads block data from a Write-Ahead Log (WAL) and persists it to a MongoDB database. It supports the following operations:

- Creating entities
- Updating entities
- Deleting entities
- Adding string annotations
- Adding numeric annotations

## Requirements

- Go 1.22 or later
- MongoDB 6.0 or later
- Access to a Golem WAL directory

## Running the ETL

```bash
go run main.go --mongo-uri "mongodb://localhost:27017" --db-name "golem" --wal "/path/to/wal" --rpc-endpoint "http://localhost:8545"
```

### Command Line Options

- `--mongo-uri`: MongoDB connection URI (required)
- `--db-name`: MongoDB database name (required)
- `--wal`: Path to the Write-Ahead Log directory (required)
- `--rpc-endpoint`: RPC endpoint for op-geth (required)

## Testing

The package includes Cucumber tests that use [Testcontainers](https://java.testcontainers.org/modules/databases/mongodb/) to spin up a MongoDB instance for testing.

To run the tests:

```bash
cd golem-base/etl/mongodb
go test -v
```

The tests use the features from `golem-base/features` along with MongoDB-specific features in `golem-base/features/mongodb`.

## MongoDB Schema

The ETL creates the following collections in MongoDB:

- `processing_status`: Tracks the last processed block
- `entities`: Stores entity data
- `string_annotations`: Stores string annotations for entities
- `numeric_annotations`: Stores numeric annotations for entities

## Development

### MongoDB Driver

The MongoDB driver implementation is in the `mongogolem` package. It provides methods for:

- Creating and managing database indexes
- CRUD operations for entities
- Managing annotations
- Tracking processing status 