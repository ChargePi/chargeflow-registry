package schema

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("schema not found")
	ErrAlreadyExists = errors.New("schema already exists")
)

type Repository interface {
	// Get retrieves a schema. If vendor/model are provided, it tries the specific
	// schema first and falls back to the generic (nil vendor/model) one.
	Get(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string) (*Schema, error)
	List(ctx context.Context, version OCPPVersion, vendor, model *string) ([]*Schema, error)
	// Add inserts a new schema. Returns ErrAlreadyExists if the exact key already exists.
	Add(ctx context.Context, schema *Schema) error
	Upsert(ctx context.Context, schema *Schema) error
	Delete(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string) error
}

type Cache interface {
	Get(ctx context.Context, key string) (*Schema, error)
	Set(ctx context.Context, key string, schema *Schema) error
	Delete(ctx context.Context, key string) error
}
