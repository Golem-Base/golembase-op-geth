# Running OP-Geth Locally with Docker Compose

This guide explains how to run OP-Geth locally with MongoDB and SQLite ETL services using Docker Compose.

## Services Overview

### Core Services

- **op-geth**: The main Ethereum node service running in dev mode
- **mongodb**: MongoDB instance configured with replica set
- **mongodb-etl**: Service for syncing blockchain data to MongoDB
- **sqlite-etl**: Service for syncing blockchain data to SQLite
- **rpcplorer**: Web interface for exploring blockchain data

### Supporting Services

- **setup**: Initializes MongoDB keyfile for replica set authentication

## Prerequisites

- Docker
- Docker Compose

## Getting Started

1. Clone the repository
2. Run the services:
   ```bash
   docker-compose up
   ```

## Service Details

### OP-Geth
- Runs in dev mode with HTTP and WebSocket APIs enabled
- Exposes port 8545 for RPC connections
- Supports various APIs: eth, web3, net, debug, golembase
- Uses write-ahead logging for data persistence

### MongoDB
- Version: 8.0.6
- Configured with replica set (rs0)
- Exposes port 27017
- Uses authentication (admin/password)
- Includes health checks for replica set status

### MongoDB ETL
- Syncs blockchain data to MongoDB
- Depends on both op-geth and MongoDB services

### SQLite ETL
- Syncs blockchain data to SQLite database
- Depends on op-geth service

### RPC Explorer
- Web interface for exploring blockchain data
- Exposes port 8080
- Connects to op-geth RPC endpoint

## Volumes

The following volumes are created and managed by Docker Compose:
- `mongodb_keyfile`: Stores MongoDB replica set keyfile
- `mongodb_data`: Persistent storage for MongoDB data
- `golembase_wal`: Write-ahead log storage
- `golembase_sqlite`: SQLite database storage
- `geth_data`: OP-Geth data storage

## Ports

- 8545: OP-Geth RPC
- 27017: MongoDB
- 8080: RPC Explorer

## Building with Latest Changes

When pulling the latest changes from the repository, you may need to rebuild the services to ensure you have the latest code:

```bash
# Stop all services
docker-compose down

# Remove existing images (optional, but recommended when pulling latest changes)
docker-compose rm -f

# Rebuild and start services
docker-compose up --build
```

## Development

To modify or extend the setup:

1. Edit `docker-compose.yml` for service configuration
2. Modify the Dockerfile for service-specific build requirements
3. Update environment variables as needed

## Troubleshooting

1. Check service logs:
   ```bash
   docker-compose logs [service-name]
   ```

2. Verify service health:
   ```bash
   docker-compose ps
   ```

3. Restart services:
   ```bash
   docker-compose restart [service-name]
   ```
