# Golem Base Storage

Golem Base provides a robust storage layer with the following characteristics:

- **Transaction-Based Storage Mutations**: All changes to storage are executed through secure transaction submissions
- **RLP-Encoded Operations**: Each transaction contains a list of operations encoded using RLP (Recursive Length Prefix) for efficient data serialization
- **Core Operation Types**:
  - CREATE: Establish new storage entries with configurable time-to-live (TTL)
  - UPDATE: Modify existing storage entries, including payload and annotations
  - DELETE: Remove storage entries completely from the system
- **Automatic Expiration**: Each block includes a housekeeping transaction that automatically removes all entities that have reached their expiration time, ensuring storage efficiency

## Format of the Storage transaction

The Golem Base Storage transaction follow the EIP-1559 transaction format with the following fields:

- `ChainID`: The chain identifier
- `Nonce`: Sender account's nonce
- `GasTipCap`: Maximum priority fee per gas (maxPriorityFeePerGas)
- `GasFeeCap`: Maximum fee per gas (maxFeePerGas)
- `Gas`: Gas limit for the transaction
- `To`: Recipient address (nil for contract creation)
- `Value`: Amount of ETH to transfer
- `Data`: Transaction payload containing RLP-encoded storage operations
- `AccessList`: EIP-2930 access list for gas optimization

The transaction also includes signature values:
- `V`, `R`, `S`: ECDSA signature components

These transactions are identified by a specific transaction type value `GolemBaseUpdateStorageTxType (0x07)` and are processed by the Golem Base subsystem to execute the storage operations contained in the Data field.

### Transaction Data

The transaction data field contains a StorageTransaction structure encoded using RLP. This structure consists of:

- `Create`: A list of Create operations, each containing:
  - `TTL`: Time-to-live in blocks, current block time of Optimism is 2 seconds.
  - `Payload`: The actual data to be stored
  - `StringAnnotations`: Key-value pairs with string values for indexing
  - `NumericAnnotations`: Key-value pairs with numeric values for indexing

- `Update`: A list of Update operations, each containing:
  - `EntityKey`: The key of the entity to update
  - `TTL`: New time-to-live in blocks
  - `Payload`: New data to replace existing payload
  - `StringAnnotations`: New string annotations
  - `NumericAnnotations`: New numeric annotations

- `Delete`: A list of entity keys (common.Hash) to be removed from storage

The transaction is atomic - all operations succeed or the entire transaction fails. Entity keys for Create operations are derived from the transaction hash, payload content, and operation index, making it unique across the whole blockchain. Annotations enable efficient querying of stored data through specialized indexes.

### Emitted Logs

When storage transactions are executed, the system emits logs to track entity lifecycle events:

- **GolemBaseStorageEntityCreated**: Emitted when a new entity is created
  - Event signature: `GolemBaseStorageEntityCreated(bytes32 entityKey, uint256 expirationBlock)`
  - Event topic: `0xce4b4ad6891d716d0b1fba2b4aeb05ec20edadb01df512263d0dde423736bbb9`
  - Topics: `[GolemBaseStorageEntityCreated, entityKey]`
  - Data: Contains the expiration block number

- **GolemBaseStorageEntityUpdated**: Emitted when an entity is updated
  - Event signature: `GolemBaseStorageEntityUpdated(bytes32 entityKey, uint256 newExpirationBlock)`
  - Event topic: `0xf371f40aa6932ad9dacbee236e5f3b93d478afe3934b5cfec5ea0d800a41d165`
  - Topics: `[GolemBaseStorageEntityUpdated, entityKey]`
  - Data: Contains the new expiration block number

- **GolemBaseStorageEntityDeleted**: Emitted when an entity is deleted
  - Event signature: `GolemBaseStorageEntityDeleted(bytes32 entityKey)`
  - Event topic: `0x0297b0e6eaf1bc2289906a8123b8ff5b19e568a60d002d47df44f8294422af93`
  - Topics: `[GolemBaseStorageEntityDeleted, entityKey]`
  - Data: Empty

These logs enable efficient tracking of storage changes and can be used by applications to monitor entity lifecycle events. The event signatures are defined as keccak256 hashes of their respective function signatures.

## Housekeeping Transaction

The Golem Base system includes an automatic housekeeping mechanism that runs during block processing to manage entity lifecycle. This process:

1. **Expires Entities**: At each block, the system identifies and removes entities whose TTL has expired
2. **Cleans Up Indexes**: When entities are deleted, their annotation indexes are automatically updated
3. **Emits Deletion Logs**: For each expired entity, a `GolemBaseStorageEntityDeleted` event is emitted

The housekeeping process is executed automatically as part of block processing, ensuring that storage remains clean and that expired data is properly removed from the system. This helps maintain system performance and ensures that temporary data doesn't persist beyond its intended lifetime.

The implementation uses a specialized index that tracks which entities expire at which block number, allowing for efficient cleanup without having to scan the entire storage space.

## JSON-RPC Namespace and Methods

The API methods are accessible through the following JSON-RPC endpoints:

- `golembase_getStorageValue`
- `golembase_getEntitiesToExpireAtBlock`
- `golembase_getEntitiesForStringAnnotationValue`
- `golembase_getEntitiesForNumericAnnotationValue`
- `golembase_queryEntities`

## API Functionality

This JSON-RPC API provides several capabilities:

1. **Storage Access**
   - `getStorageValue`: Retrieves payload data for a given hash key

2. **Entity Queries**
   - `getEntitiesToExpireAtBlock`: Returns entities scheduled to expire at a specific block
   - `getEntitiesForStringAnnotationValue`: Finds entities with matching string annotations
   - `getEntitiesForNumericAnnotationValue`: Finds entities with matching numeric annotations

3. **Query Language Support**
   - `queryEntities`: Executes queries with a custom query language, returning structured results

