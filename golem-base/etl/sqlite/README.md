# SQLite ETL

This program implements an Extract, Transform, Load (ETL) process that processes blockchain data from the Write-Ahead Log (WAL) written by the Golem Base extension of op-geth. The WAL contains entity operations (create, update, delete) and their associated annotations, which this program processes and stores in a SQLite database.

## Features

- Processes blockchain data from Golem Base WAL files
- Stores entity data and annotations in SQLite
- Supports numeric and string annotations for entities
- Handles entity lifecycle operations (create, update, delete)
- Maintains processing status to track progress

## Requirements

- Go 1.x
- SQLite3
- Access to an op-geth RPC endpoint
- op-geth configured with Golem Base extension

## op-geth Configuration

To enable WAL writing in op-geth with Golem Base extension, you need to:

1. Start op-geth with the `--golembase.writeaheadlog` flag pointing to your desired WAL directory:
```bash
op-geth --golembase.writeaheadlog /path/to/wal/directory
```

2. Make sure the WAL directory exists and is writable by the op-geth process.

The WAL directory will contain files that record all entity operations processed by the Golem Base extension. These files are what the SQLite ETL program processes.

## Configuration

The program requires the following configuration parameters:

- `--db`: Path to the SQLite database file (required)
- `--wal`: Directory containing the Write-Ahead Log files (required)
- `--rpc-endpoint`: URL of the op-geth RPC endpoint (required)

These can be provided via command line flags or environment variables:
- `DB_FILE`
- `WAL_DIR`
- `RPC_ENDPOINT`

## Usage

```bash
sqlite-etl --db ./data.db --wal ./wal --rpc-endpoint http://localhost:8545
```

## Database Schema

The program uses a SQLite database with the following main tables:

- `entities`: Stores the main entity data
- `numeric_annotations`: Stores numeric annotations for entities
- `string_annotations`: Stores string annotations for entities
- `processing_status`: Tracks the last processed block

## Processing Flow

1. Connects to the op-geth RPC endpoint
2. Checks for existing processing status
3. If no status exists, initializes with genesis block
4. Processes WAL files sequentially
5. For each block:
   - Processes all operations (create, update, delete)
   - Handles entity data and annotations
   - Updates processing status
6. Uses transactions to ensure data consistency

## Error Handling

- Graceful shutdown on interrupt signals
- Transaction rollback on processing errors
- Detailed error logging
- Automatic schema initialization if needed
