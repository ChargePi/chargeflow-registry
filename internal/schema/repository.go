package schema

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("schema not found")
	ErrAlreadyExists = errors.New("schema already exists")
)

const (
	// DefaultPageSize is applied when a caller requests a paginated listing with limit 0.
	DefaultPageSize = 50
	// MaxPageSize caps the number of rows returned in a single paginated call.
	MaxPageSize = 200
)

type Repository interface {
	// Get retrieves a schema. If vendor/model are provided, it tries the specific
	// schema first and falls back to the generic (nil vendor/model) one.
	Get(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string) (*Schema, error)
	// List returns schemas matching the given filters, paginated. An empty version,
	// nil vendor/model, or nil status is treated as "any" for that field. Returns
	// the page of results plus the total count of matching rows, ignoring
	// limit/offset.
	List(ctx context.Context, version OCPPVersion, vendor, model *string, status *Status, limit, offset uint32) ([]*Schema, int64, error)
	// Add inserts a new schema. Returns ErrAlreadyExists if the exact key already exists.
	Add(ctx context.Context, schema *Schema) error
	Upsert(ctx context.Context, schema *Schema) error
	Delete(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string) error
	// UpdateStatus sets the status of the schema with the given ID and returns the
	// updated schema. Returns ErrNotFound if no schema with that ID exists.
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) (*Schema, error)
}

type Cache interface {
	Get(ctx context.Context, key string) (*Schema, error)
	Set(ctx context.Context, key string, schema *Schema) error
	Delete(ctx context.Context, key string) error
}
