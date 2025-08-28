-- name: InsertEntity :exec
INSERT INTO entities (key, expires_at, payload) VALUES (?, ?, ?);

-- name: InsertStringAnnotation :exec
INSERT INTO annotations (entity_key, annotation_key, string_value) VALUES (?, ?, ?);

-- name: InsertNumericAnnotation :exec
INSERT INTO annotations (entity_key, annotation_key, numeric_value) VALUES (?, ?, ?);

-- name: GetEntity :one
SELECT e.expires_at, e.payload, a.string_value AS owner_address
FROM entities e INNER JOIN annotations a
  ON e.key = a.entity_key AND a.annotation_key = "$owner"
WHERE key = ?;

-- name: GetEntityPayload :one
SELECT payload FROM entities WHERE key = ?;

-- name: GetEntitiesByOwner :many
SELECT e.key, e.expires_at, e.payload
FROM entities e INNER JOIN annotations a
  ON e.key = a.entity_key AND a.annotation_key = "$owner"
WHERE a.string_value = ?;

-- name: GetEntityKeysByOwner :many
SELECT e.key
FROM entities e INNER JOIN annotations a
  ON e.key = a.entity_key AND a.annotation_key = "$owner"
WHERE a.string_value = ? ORDER BY e.key;

-- name: GetStringAnnotations :many
SELECT annotation_key, string_value
FROM annotations
WHERE entity_key = ? AND string_value IS NOT NULL;

-- name: GetNumericAnnotations :many
SELECT annotation_key, numeric_value
FROM annotations
WHERE entity_key = ? AND numeric_value IS NOT NULL;

-- name: DeleteEntity :exec
DELETE FROM entities WHERE key = ?;

-- name: DeleteStringAnnotations :exec
DELETE FROM annotations WHERE entity_key = ? AND string_value IS NOT NULL;

-- name: DeleteNumericAnnotations :exec
DELETE FROM annotations WHERE entity_key = ? AND numeric_value IS NOT NULL;

-- name: DeleteAnnotations :exec
DELETE FROM annotations WHERE entity_key = ?;

-- name: UpdateEntityExpiresAt :exec
UPDATE entities SET expires_at = ? WHERE key = ?;

-- name: GetProcessingStatus :one
SELECT last_processed_block_number, last_processed_block_hash FROM processing_status WHERE network = ?;

-- name: UpdateProcessingStatus :exec
UPDATE processing_status SET last_processed_block_number = ?, last_processed_block_hash = ? WHERE network = ?;

-- name: InsertProcessingStatus :exec
INSERT INTO processing_status (network, last_processed_block_number, last_processed_block_hash) VALUES (?, ?, ?);

-- name: HasProcessingStatus :one
SELECT COUNT(*) > 0 FROM processing_status WHERE network = ?;

-- name: CountNetworks :one
SELECT COUNT(DISTINCT network) FROM processing_status;

-- name: DeleteProcessingStatus :exec
DELETE FROM processing_status WHERE network = ?;

-- name: EntityExists :one
SELECT COUNT(*) > 0 FROM entities WHERE key = ?;

-- name: StringAnnotationsForEntityExists :one
SELECT COUNT(*) > 0 FROM annotations WHERE entity_key = ? AND string_value IS NOT NULL;

-- name: NumericAnnotationsForEntityExists :one
SELECT COUNT(*) > 0 FROM annotations WHERE entity_key = ? AND numeric_value IS NOT NULL;

-- name: DeleteAllEntities :exec
DELETE FROM entities;

-- name: DeleteAllStringAnnotations :exec
DELETE FROM annotations WHERE string_value IS NOT NULL;

-- name: DeleteAllNumericAnnotations :exec
DELETE FROM annotations WHERE numeric_value IS NOT NULL;

-- name: DeleteAllAnnotations :exec
DELETE FROM annotations;

-- name: DeleteAllProcessingStatus :exec
DELETE FROM processing_status;

-- name: GetEntityMetadata :one
SELECT
  e.expires_at,
  a.string_value AS owner_address,
  e.payload
FROM entities e INNER JOIN annotations a
  ON e.key = a.entity_key AND a.annotation_key = "$owner"
WHERE e.key = ?;

-- name: GetEntityAnnotations :many
SELECT
  annotation_key,
  string_value,
  numeric_value
FROM annotations
WHERE entity_key = ?
ORDER BY annotation_key;

-- name: GetEntityStringAnnotations :many
SELECT
  annotation_key,
  string_value
FROM annotations
WHERE entity_key = ? AND string_value IS NOT NULL
ORDER BY annotation_key;

-- name: GetEntityNumericAnnotations :many
SELECT
  annotation_key,
  numeric_value
FROM annotations
WHERE entity_key = ? AND numeric_value IS NOT NULL
ORDER BY annotation_key;

-- name: GetEntitiesToExpireAtBlock :many
SELECT key
FROM entities
WHERE expires_at = ?
ORDER BY key;

-- name: GetEntitiesForStringAnnotation :many
SELECT entity_key
FROM annotations
WHERE annotation_key = ? AND string_value = ?
ORDER BY entity_key;

-- name: GetEntitiesForNumericAnnotation :many
SELECT entity_key
FROM annotations
WHERE annotation_key = ? AND numeric_value = ?
ORDER BY entity_key;

-- name: GetAllEntityKeys :many
SELECT key FROM entities ORDER BY key;

-- name: GetEntityCount :one
SELECT COUNT(*) FROM entities;
