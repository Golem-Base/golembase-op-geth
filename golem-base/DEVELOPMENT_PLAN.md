# Development Plan

This document is a high-level plan for the development of the golem-base prototype.
It has more details than the scope of the prototype so it can be used as a reference for the future development.

## Scope

- Updating/Creating of the data in the storage using transactions
- Deleting of the data in the storage using transactions
- Expiry of the data in the storage after TTL.
- Querying of the data in the storage / prototype of the query language
- Storage using SQLite database

## Not in the scope
- Pruning of the transaction history
- Implementation of the p2p sync
- L2/L3 Consensus
- Events

## Current approach
### Adding a new transaction type `UpdateStorageTx`

Payload of the transaction is a list of `Create`, `Update`, `Delete` or `Extend` operations

- The payload of the transaction is compressed using zStd, ideally using best compression level. Any compression level is fine as long as we can decompress it on the node.
    - `Idea`: we could charge per byte of the transaction data, incentivizing using the best compression level

- Each created payload is assigned an unique ID

### Transaction Processing

- We can use `Keccak256(payload ++ transaction hash ++ index of the operation in the transaction)` for the ID of newly created payloads

- Each `Create` operation contains a TTL value, which is the number of blocks after which the payload expires

- Each `Extend` operation contains a key of the record and TTL value, which is the number of blocks after which the payload expires

- `Update` and `Delete` operations are just references to the existing payloads. If the payload is not found, the whole transaction is failed and will be stored as such in the block. The sender of the Tx will be charged for the processing of the transaction (`TBD`: how much? Depending on the number of operations in the transaction?)

- Proper semantic of the TTL, expiriation and extension of the TTL are `TBD`

- Transaction receipt should be extended with a field of created and updated payloads

- We should store the records in the nodes internal storage when processing the transaction. This should align nicely with sync of the nodes.

- Each Create Operation emits an event (log) of the following format:
    ```
    GolemBaseStorageEntityCreated(uint256,uint256)
    ```
    The values are the hash of the payload (a.k.a the ID of the payload) and the block number when the payload will expire.

- Each create operation stores/updates index for each of the annotations provided in the payload.
    - For the `string` type of the annotation, following will be stored
        - key: `Keccak256('golemBaseStringAnnotation',key,value)`
        - value: list if entity keys that have this `key=value` set.
    - For the `numeric` type of the annotations following is stored
        - key `Keccak256('golemBaseNumericAnnotation',key,value)`, `value` is Big Endian encoded 64-bit (8 byte) integer.
        - value: list if entity keys that have this `key=value` set.
    Whenever we store the entity in the list, we append the key of the entity to the end of the list.


  
