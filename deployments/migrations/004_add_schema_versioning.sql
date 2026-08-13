-- +goose Up
ALTER TABLE schemas
    ADD COLUMN version INT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS schema_versions (
    id         UUID        NOT NULL DEFAULT gen_random_uuid(),
    schema_id  UUID        NOT NULL REFERENCES schemas (id) ON DELETE CASCADE,
    version    INT         NOT NULL,
    schema     JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id),
    CONSTRAINT schema_versions_unique UNIQUE (schema_id, version)
);

CREATE INDEX IF NOT EXISTS idx_schema_versions_schema_id ON schema_versions (schema_id);

-- Backfill: every schema that predates version tracking (including the OCPP 1.6
-- seed rows from 003) gets a version=1 changelog entry reflecting its current
-- content. updated_at is used as created_at here since it's the closest
-- available proxy for "when this content became active" on rows that may have
-- already gone through an Upsert before this migration existed.
INSERT INTO schema_versions (schema_id, version, schema, created_at)
SELECT id, 1, schema, updated_at FROM schemas
ON CONFLICT ON CONSTRAINT schema_versions_unique DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS schema_versions;
ALTER TABLE schemas DROP COLUMN version;
