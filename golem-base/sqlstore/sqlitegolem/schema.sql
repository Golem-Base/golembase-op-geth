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

CREATE TABLE annotations (
  entity_key TEXT NOT NULL,
  annotation_key TEXT NOT NULL,
  string_value TEXT,
  numeric_value INTEGER,
  PRIMARY KEY (entity_key, annotation_key)
);
