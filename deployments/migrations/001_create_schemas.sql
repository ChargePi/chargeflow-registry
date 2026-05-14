-- +goose Up
CREATE TABLE IF NOT EXISTS schemas (
    id           UUID        NOT NULL DEFAULT gen_random_uuid(),
    ocpp_version VARCHAR(10) NOT NULL,
    action       VARCHAR(100) NOT NULL,
    message_type VARCHAR(10) NOT NULL,
    vendor       VARCHAR(100),
    model        VARCHAR(100),
    schema       JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id),
    CONSTRAINT schemas_lookup_unique
        UNIQUE NULLS NOT DISTINCT (ocpp_version, action, message_type, vendor, model)
);

CREATE INDEX IF NOT EXISTS idx_schemas_version ON schemas (ocpp_version);
CREATE INDEX IF NOT EXISTS idx_schemas_vendor_model ON schemas (vendor, model);

-- +goose Down
DROP TABLE IF EXISTS schemas;