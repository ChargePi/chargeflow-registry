-- +goose Up
ALTER TABLE schemas
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'submitted';

ALTER TABLE schemas
    ADD CONSTRAINT schemas_status_check CHECK (status IN ('submitted', 'verified', 'rejected'));

CREATE INDEX IF NOT EXISTS idx_schemas_status ON schemas (status);

-- +goose Down
DROP INDEX IF EXISTS idx_schemas_status;
ALTER TABLE schemas DROP CONSTRAINT schemas_status_check;
ALTER TABLE schemas DROP COLUMN status;
