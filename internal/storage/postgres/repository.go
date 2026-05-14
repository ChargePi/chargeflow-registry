package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"gorm.io/gorm"
)

type SchemaRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *SchemaRepository {
	return &SchemaRepository{db: db}
}

func (r *SchemaRepository) Get(ctx context.Context, version schema.OCPPVersion, action string, msgType schema.MessageType, vendor, model *string) (*schema.Schema, error) {
	if vendor != nil || model != nil {
		var entity schemaEntity
		err := r.db.WithContext(ctx).
			Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NOT DISTINCT FROM ? AND model IS NOT DISTINCT FROM ?",
				version, action, msgType, vendor, model).
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
		Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NULL AND model IS NULL",
			version, action, msgType).
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
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return schema.ErrAlreadyExists
		}
		return fmt.Errorf("add schema: %w", err)
	}

	s.ID = entity.ID
	return nil
}

func (r *SchemaRepository) List(ctx context.Context, version schema.OCPPVersion, vendor, model *string) ([]*schema.Schema, error) {
	var entities []*schemaEntity
	err := r.db.WithContext(ctx).
		Where("ocpp_version = ? AND vendor IS NOT DISTINCT FROM ? AND model IS NOT DISTINCT FROM ?",
			version, vendor, model).
		Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return toDomainSlice(entities), nil
}

func (r *SchemaRepository) Upsert(ctx context.Context, s *schema.Schema) error {
	entity := toEntity(s)
	err := r.db.WithContext(ctx).
		Where("ocpp_version = ? AND action = ? AND message_type = ? AND vendor IS NOT DISTINCT FROM ? AND model IS NOT DISTINCT FROM ?",
			s.OCPPVersion, s.Action, s.MessageType, s.Vendor, s.Model).
		Assign(schemaEntity{Schema: s.Schema}).
		FirstOrCreate(entity).Error
	if err != nil {
		return fmt.Errorf("upsert schema: %w", err)
	}

	s.ID = entity.ID
	return nil
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
