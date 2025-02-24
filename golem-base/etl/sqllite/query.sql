-- name: InsertEntity :exec
INSERT INTO entities (key, expires_at, payload) VALUES (?, ?, ?);

-- name: InsertStringAnnotation :exec
INSERT INTO string_annotations (entity_key, annotation_key, value) VALUES (?, ?, ?);

-- name: InsertNumericAnnotation :exec
INSERT INTO numeric_annotations (entity_key, annotation_key, value) VALUES (?, ?, ?);

-- name: GetEntity :one
SELECT expires_at, payload FROM entities WHERE key = ?;

-- name: GetStringAnnotations :many
SELECT annotation_key, value FROM string_annotations WHERE entity_key = ?;

-- name: GetNumericAnnotations :many
SELECT annotation_key, value FROM numeric_annotations WHERE entity_key = ?;

-- name: DeleteEntity :exec
DELETE FROM entities WHERE key = ?;

-- name: DeleteStringAnnotations :exec
DELETE FROM string_annotations WHERE entity_key = ?;

-- name: DeleteNumericAnnotations :exec
DELETE FROM numeric_annotations WHERE entity_key = ?;

-- name: GetProcessingStatus :one
SELECT last_processed_block FROM processing_status WHERE network = ?;

-- name: UpdateProcessingStatus :exec
UPDATE processing_status SET last_processed_block = ? WHERE network = ?;

-- name: InsertProcessingStatus :exec
INSERT INTO processing_status (network, last_processed_block) VALUES (?, ?);

-- name: HasProcessingStatus :one
SELECT COUNT(*) > 1 FROM processing_status WHERE network = ?;

-- name: DeleteProcessingStatus :exec
DELETE FROM processing_status WHERE network = ?;
