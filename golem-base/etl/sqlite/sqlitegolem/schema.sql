CREATE TABLE processing_status (
  network TEXT NOT NULL PRIMARY KEY,
  last_processed_block_number INTEGER NOT NULL,
  last_processed_block_hash TEXT NOT NULL
);

CREATE TABLE entities (
  key TEXT NOT NULL PRIMARY KEY,
  expires_at INTEGER NOT NULL,
  payload BLOB NOT NULL
);

CREATE TABLE string_annotations (
  entity_key TEXT NOT NULL,
  annotation_key TEXT NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY (entity_key, annotation_key)
);

CREATE TABLE numeric_annotations (
  entity_key TEXT NOT NULL,
  annotation_key TEXT NOT NULL,
  value INTEGER NOT NULL,
  PRIMARY KEY (entity_key, annotation_key)
);

