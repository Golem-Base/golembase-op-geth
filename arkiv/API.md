# Arkiv API

Arkiv is a decentralized storage system built into the Ethereum blockchain that allows for the creation, management, and querying of entities with metadata and expiration times. This API provides comprehensive access to storage operations and data retrieval.

## Overview

Arkiv operates through two main mechanisms:
1. **Transaction-based mutations** - Creating, updating, and deleting entities
2. **RPC-based queries** - Retrieving and searching stored data

All stored entities have:
- **Unique hash keys** - Automatically generated from content and transaction data
- **Expiration (BTL)** - Block-to-live counter determining when data expires
- **Annotations** - Key-value metadata for indexing and searching
- **Ownership** - Ethereum address that controls the entity
- **Content type** - MIME-type identifier for the stored data

## Transaction Operations

All create/update/delete operations are sent as Ethereum transactions to the Arkiv processor address: `0x00000000000000000000000000000061726B6976`

### RLP Type System

The following primitive types are used in RLP encoding:
- **Uint64**: 64-bit unsigned integer
- **String**: UTF-8 encoded string
- **Bytes**: Raw byte array
- **Hash32**: 32-byte hash value
- **Address**: 20-byte Ethereum address
- **List[T]**: Array/list of type T

The transaction payload must be RLP encoded with the following structure:

**RLP Structure:**
```
ArkivTransaction [
    Create:      List[ArkivCreate],      // Create new entities
    Update:      List[ArkivUpdate],      // Update existing entities
    Delete:      List[Hash32],           // Delete entities by key
    Extend:      List[ExtendBTL],        // Extend entity expiration
    ChangeOwner: List[ArkivChangeOwner]  // Transfer ownership
]
```

### Create Operations

Creates new entities in the storage system. Each entity gets a unique key derived from the payload content, transaction hash, and operation index.

**RLP Structure:**
```
ArkivCreate [
    BTL:                Uint64,                    // Blocks to live (expiration)
    ContentType:        String,                    // MIME type (max 128 chars)
    Payload:            Bytes,                     // The actual data to store
    StringAnnotations:  List[StringAnnotation],    // String metadata
    NumericAnnotations: List[NumericAnnotation]    // Numeric metadata
]

StringAnnotation [
    Key:   String,  // Annotation key
    Value: String   // Annotation value
]

NumericAnnotation [
    Key:   String,  // Annotation key
    Value: Uint64   // Numeric value
]
```

**Requirements:**
- `BTL` must be > 0 (determines when entity expires)
- `ContentType` must be non-empty and ≤ 128 characters
- Annotation keys must match the pattern `[\p{L}_][\p{L}\p{N}_]*` and be unique within their type
- Transaction sender becomes the entity owner

**Annotation Key Rules:**
- Must start with a Unicode letter (`\p{L}`) or underscore (`_`)
- Can contain Unicode letters (`\p{L}`), Unicode numbers (`\p{N}`), or underscores (`_`)
- Cannot start with `$` (reserved for meta-fields) or `0x` (reserved for addresses/hashes)
- Examples: `type`, `name_with_underscore`, `_starts_with_underscore`, `version123`, `αβγ` (Unicode)

### Update Operations

Updates existing entities. Only the entity owner can perform updates.

**RLP Structure:**
```
ArkivUpdate [
    EntityKey:          Hash32,                    // Hash of entity to update
    ContentType:        String,                    // New content type
    BTL:                Uint64,                    // New expiration time
    Payload:            Bytes,                     // New payload data
    StringAnnotations:  List[StringAnnotation],    // New string annotations
    NumericAnnotations: List[NumericAnnotation]    // New numeric annotations
]
```

**Requirements:**
- Entity must exist and be owned by transaction sender
- Same annotation key rules and validation as create operations apply
- Updates reset the expiration time based on current block + BTL

### Delete Operations

Removes entities from storage. Only the entity owner can delete their entities.

**RLP Structure:**
```
Delete: List[Hash32]  // Array of entity keys to delete
```

### Extend BTL Operations

Extends the expiration time of existing entities without modifying their content.

**RLP Structure:**
```
ExtendBTL [
    EntityKey:      Hash32,  // Hash of entity to extend
    NumberOfBlocks: Uint64   // Additional blocks to add
]
```

### Change Owner Operations

Transfers ownership of entities to a new Ethereum address.

**RLP Structure:**
```
ArkivChangeOwner [
    EntityKey: Hash32,   // Hash of entity to transfer
    NewOwner:  Address   // New owner address (20 bytes)
]
```

## Query API

The query system provides powerful search capabilities through JSON-RPC methods under the `arkiv` namespace.

### Primary Query Method

#### `arkiv_query`

Executes complex queries using a custom query language.

**Parameters:**
- `query` (string): Query expression using Arkiv query language
- `options` (QueryOptions, optional): Query configuration

**JSON Structure:**
```json
{
  "atBlock": number,        // Query at specific block (optional, default: latest)
  "includeData": {          // Control returned data fields (optional)
    "key": boolean,         // Include entity key
    "annotations": boolean, // Include annotation metadata
    "payload": boolean,     // Include stored data
    "expiration": boolean,  // Include expiration info
    "owner": boolean        // Include owner address
  },
  "resultsPerPage": number, // Pagination limit (optional)
  "cursor": string          // Pagination cursor from previous response (optional)
}
```

**Returns:**
**JSON Response:**
```json
{
  "blockNumber": number,  // Block number queried
  "data": [...],          // Array of matching entities
  "cursor": string        // Next pagination cursor (only present if more data available)
}
```

#### Pagination Behavior

The `cursor` field in the response is **only included when there is more data to be fetched**. Pagination is triggered by either:

1. **Results per page limit**: When the number of results reaches the `resultsPerPage` value (if specified)
2. **Response size limit**: When the total response size approaches the maximum allowed size (256KB)

If no `cursor` is returned, it means all matching results have been retrieved. To continue pagination, include the returned `cursor` value in the next request.

### Query Language

The Arkiv query language supports complex filtering and searching:

#### Basic Syntax

- **String annotations**: `name = "example"`
- **Numeric annotations**: `age = 30`
- **Meta-fields**: `$owner = "0x1234..."`, `$key = "0xabcd..."`, `$expiration > 1000`

#### Operators

- **Equality**: `=`, `!=`
- **Comparison**: `<`, `>`, `<=`, `>=`
- **Pattern matching**: `~` (glob), `!~` (not glob)
- **Logical**: `&&` (AND), `||` (OR)
- **Grouping**: `()` for precedence

#### Examples

```javascript
// Find entities by string annotation
'name = "document"'

// Combine conditions
'type = "image" && status = "approved"'

// Use meta-fields
'$owner = "0x742d35Cc6634C0532925a3b8D80C6Fb8C3" && $expiration > 2000'

// Complex queries with grouping
'(category = "public" || category = "shared") && $owner = "0x123..."'

// Pattern matching
'filename ~ "*.pdf"'
```


## Usage Examples

### Creating an Entity

```javascript
// Prepare transaction data
const arkivTx = {
    create: [{
        btl: 1000,                    // Expires in 1000 blocks
        contentType: "application/json",
        payload: Buffer.from(JSON.stringify({data: "example"})),
        stringAnnotations: [{
            key: "category",
            value: "document"
        }],
        numericAnnotations: [{
            key: "version",
            value: 1
        }]
    }],
    update: [],
    delete: [],
    extend: [],
    changeOwner: []
};

// Send transaction to Arkiv processor address
const tx = await web3.eth.sendTransaction({
    to: "0x00000000000000000000000000000061726B6976",
    data: rlp.encode(arkivTx),
    from: senderAddress
});
```

### Querying Data

```javascript
// Simple query
const result = await web3.eth.call({
    method: "arkiv_query",
    params: [
        'category = "document"',
        {
            includeData: {
                key: true,
                payload: true,
                annotations: true
            },
            resultsPerPage: 10
        }
    ]
});

// Complex query with pagination
const complexResult = await web3.eth.call({
    method: "arkiv_query", 
    params: [
        '(status = "active" || status = "pending") && $expiration > 2000',
        {
            atBlock: 1500000,
            cursor: "encoded_cursor_string",
            resultsPerPage: 50,
            includeData: {
                key: true,
                payload: false,
                owner: true,
                expiration: true
            }
        }
    ]
});
```

## Error Handling

Common error conditions:
- **Validation errors**: Invalid BTL, content type, or annotation format
- **Permission errors**: Attempting to modify entities not owned by sender
- **Not found errors**: Referencing non-existent entities
- **Query syntax errors**: Invalid query language syntax
- **Resource limits**: Response size or pagination limits exceeded

## Transaction Atomicity

All operations within an `ArkivTransaction` are atomic - either all operations succeed or all fail. This ensures data consistency across complex multi-operation transactions.