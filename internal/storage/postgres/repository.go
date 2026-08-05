package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SchemaRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *SchemaRepository {
	return &SchemaRepository{db: db}
}

// Get retrieves a verified schema. Schemas awaiting review or rejected by an admin
// are only visible through the AdminAPI, not this general lookup path.
func (r *SchemaRepository) Get(ctx context.Context, version schema.OCPPVersion, action string, msgType schema.MessageType, vendor, model *string) (*schema.Schema, error) {
	if vendor != nil || model != nil {
		var entity schemaEntity
		err := r.db.WithContext(ctx).
			Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NOT DISTINCT FROM ? AND model IS NOT DISTINCT FROM ? AND status = ?",
				version, action, msgType, vendor, model, string(schema.StatusVerified)).
			First(&entity).Error
		if err == nil {
			return toDomain(&entity), nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get schema: %w", err)
		}
	}

	// Fall back to generic schema (nil vendor/model)
	var entity schemaEntity
	err := r.db.WithContext(ctx).
		Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NULL AND model IS NULL AND status = ?",
			version, action, msgType, string(schema.StatusVerified)).
		First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, schema.ErrNotFound
		}
		return nil, fmt.Errorf("get schema: %w", err)
	}

	return toDomain(&entity), nil
}

func (r *SchemaRepository) Add(ctx context.Context, s *schema.Schema) error {
	entity := toEntity(s)
	entity.Version = 1

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		return tx.Create(&schemaVersionEntity{
			SchemaID: entity.ID,
			Version:  entity.Version,
			Schema:   entity.Schema,
		}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return schema.ErrAlreadyExists
		}
		return fmt.Errorf("add schema: %w", err)
	}

	s.ID = entity.ID
	s.Version = entity.Version
	return nil
}

// List returns schemas matching the given filters, paginated. An empty version, nil
// vendor/model/action/status, or nil msgType matches "any" for that field.
func (r *SchemaRepository) List(ctx context.Context, version schema.OCPPVersion, vendor, model, action *string, msgType *schema.MessageType, status *schema.Status, limit, offset uint32) ([]*schema.Schema, int64, error) {
	query := r.db.WithContext(ctx).Model(&schemaEntity{})
	if version != "" {
		query = query.Where("ocpp_version = ?", version)
	}
	if vendor != nil {
		query = query.Where("vendor = ?", *vendor)
	}
	if model != nil {
		query = query.Where("model = ?", *model)
	}
	if action != nil {
		query = query.Where("action = ?", *action)
	}
	if msgType != nil {
		query = query.Where("message_type = ?", string(*msgType))
	}
	if status != nil {
		query = query.Where("status = ?", string(*status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count schemas: %w", err)
	}

	var entities []*schemaEntity
	if err := query.Order("ocpp_version ASC, vendor ASC, model ASC, action ASC").Limit(int(limit)).Offset(int(offset)).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("list schemas: %w", err)
	}

	return toDomainSlice(entities), total, nil
}

// UpdateStatus sets the status of the schema with the given ID and returns the updated schema.
func (r *SchemaRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status schema.Status) (*schema.Schema, error) {
	result := r.db.WithContext(ctx).Model(&schemaEntity{}).Where("id = ?", id).Update("status", string(status))
	if result.Error != nil {
		return nil, fmt.Errorf("update schema status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, schema.ErrNotFound
	}

	var entity schemaEntity
	if err := r.db.WithContext(ctx).First(&entity, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get updated schema: %w", err)
	}

	return toDomain(&entity), nil
}

// Upsert inserts a new schema at version 1, or updates an existing one's content
// and appends a changelog entry to schema_versions. If the incoming content is
// byte-identical to what's already stored, the version and changelog are left
// untouched. Status is never modified by an update, matching the previous
// Assign-based behavior.
func (r *SchemaRepository) Upsert(ctx context.Context, s *schema.Schema) error {
	var err error
	// Two attempts: if the "row doesn't exist yet" branch loses a race to a
	// concurrent Upsert on the same logical key (both see no row, both try to
	// insert), the loser hits schemas_lookup_unique. Retrying re-reads and takes
	// the update path against the row the winner just created.
	for attempt := 0; attempt < 2; attempt++ {
		err = r.upsertOnce(ctx, s)
		if err == nil || !errors.Is(err, gorm.ErrDuplicatedKey) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("upsert schema: %w", err)
	}
	return nil
}

func (r *SchemaRepository) upsertOnce(ctx context.Context, s *schema.Schema) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing schemaEntity
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NOT DISTINCT FROM ? AND model IS NOT DISTINCT FROM ?",
				s.OCPPVersion, s.Action, s.MessageType, s.Vendor, s.Model).
			First(&existing).Error

		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			entity := toEntity(s)
			entity.Version = 1
			if err := tx.Create(entity).Error; err != nil {
				return err
			}
			if err := tx.Create(&schemaVersionEntity{
				SchemaID: entity.ID,
				Version:  entity.Version,
				Schema:   entity.Schema,
			}).Error; err != nil {
				return err
			}
			s.ID = entity.ID
			s.Version = entity.Version
			return nil
		case err != nil:
			return err
		}

		if jsonContentEqual(existing.Schema, s.Schema) {
			s.ID = existing.ID
			s.Version = existing.Version
			return nil
		}

		newVersion := existing.Version + 1
		if err := tx.Model(&schemaEntity{}).Where("id = ?", existing.ID).
			Updates(map[string]any{"schema": s.Schema, "version": newVersion}).Error; err != nil {
			return err
		}
		if err := tx.Create(&schemaVersionEntity{
			SchemaID: existing.ID,
			Version:  newVersion,
			Schema:   s.Schema,
		}).Error; err != nil {
			return err
		}

		s.ID = existing.ID
		s.Version = newVersion
		return nil
	})
}

// jsonContentEqual reports whether a and b encode the same JSON document,
// ignoring formatting differences. This is necessary because jsonb columns
// are re-serialized by Postgres on write (whitespace stripped, keys and
// numbers reformatted), so a raw byte comparison between a freshly-read row
// and an incoming request would almost always report "different" even for
// semantically identical content.
func jsonContentEqual(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

// ListVersions returns the content changelog for a schema, newest first,
// paginated. A nonexistent schemaID simply yields an empty page.
func (r *SchemaRepository) ListVersions(ctx context.Context, schemaID uuid.UUID, limit, offset uint32) ([]*schema.SchemaVersion, int64, error) {
	query := r.db.WithContext(ctx).Model(&schemaVersionEntity{}).Where("schema_id = ?", schemaID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count schema versions: %w", err)
	}

	var entities []*schemaVersionEntity
	if err := query.Order("version DESC").Limit(int(limit)).Offset(int(offset)).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("list schema versions: %w", err)
	}

	return toVersionDomainSlice(entities), total, nil
}

// GetVersion retrieves a single historical version's content for a schema.
func (r *SchemaRepository) GetVersion(ctx context.Context, schemaID uuid.UUID, version int) (*schema.SchemaVersion, error) {
	var entity schemaVersionEntity
	err := r.db.WithContext(ctx).
		Where("schema_id = ? AND version = ?", schemaID, version).
		First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, schema.ErrNotFound
		}
		return nil, fmt.Errorf("get schema version: %w", err)
	}

	return toVersionDomain(&entity), nil
}

// ListVendorModels returns the distinct OCPP version/vendor/model combinations for
// verified schemas belonging to vendor, optionally filtered to models, paginated.
// Entries without a model (generic vendor-only schemas) are excluded, since these
// represent hardware charge point models rather than CSMS vendors.
func (r *SchemaRepository) ListVendorModels(ctx context.Context, vendor string, models []string, limit, offset uint32) ([]*schema.VendorModel, int64, error) {
	query := r.db.WithContext(ctx).Model(&schemaEntity{}).
		Distinct("ocpp_version", "vendor", "model").
		Where("vendor = ? AND model IS NOT NULL AND status = ?", vendor, string(schema.StatusVerified))
	if len(models) > 0 {
		query = query.Where("model IN ?", models)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count vendor models: %w", err)
	}
	if total == 0 {
		return nil, 0, schema.ErrNotFound
	}

	var entities []*schemaEntity
	if err := query.Order("ocpp_version, model").Limit(int(limit)).Offset(int(offset)).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("list vendor models: %w", err)
	}

	vendorModels := make([]*schema.VendorModel, len(entities))
	for i, e := range entities {
		vendorModels[i] = &schema.VendorModel{
			OCPPVersion: schema.OCPPVersion(e.OCPPVersion),
			Vendor:      *e.Vendor,
			Model:       *e.Model,
		}
	}

	return vendorModels, total, nil
}

func (r *SchemaRepository) Delete(ctx context.Context, version schema.OCPPVersion, action string, msgType schema.MessageType, vendor, model *string) error {
	result := r.db.WithContext(ctx).
		Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NOT DISTINCT FROM ? AND model IS NOT DISTINCT FROM ?",
			version, action, msgType, vendor, model).
		Delete(&schemaEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete schema: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return schema.ErrNotFound
	}

	return nil
}
