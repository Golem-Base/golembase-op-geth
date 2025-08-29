CREATE TABLE processing_status (
  network TEXT NOT NULL PRIMARY KEY,
  last_processed_block_number INTEGER NOT NULL,
  last_processed_block_hash TEXT NOT NULL
);

CREATE TABLE entities (
  key TEXT NOT NULL PRIMARY KEY,
  expires_at INTEGER NOT NULL,
  payload BLOB NOT NULL,
  owner_address TEXT NOT NULL
);

CREATE INDEX idx_entities_owner_address ON entities(owner_address);

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

-- For numeric_annotations direct lookups 
CREATE INDEX idx_numeric_annotations_key_value
ON numeric_annotations(annotation_key, value, entity_key);

-- For numeric_annotations range lookups 
CREATE INDEX idx_numeric_annotations_range
ON numeric_annotations(annotation_key, value);

-- For string_annotations lookups 
CREATE INDEX idx_string_annotations_key_value
ON string_annotations(annotation_key, value, entity_key);

-- For string_annotations range lookups 
CREATE INDEX idx_string_annotations_range
ON string_annotations(annotation_key, value);
