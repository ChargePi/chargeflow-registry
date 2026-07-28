package postgres

import (
	"encoding/json"
	"time"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/google/uuid"
)

type schemaEntity struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OCPPVersion string          `gorm:"column:ocpp_version;not null;index:idx_schemas_version;index:idx_schemas_lookup,priority:1"`
	Action      string          `gorm:"column:action;not null;index:idx_schemas_lookup,priority:2"`
	MessageType string          `gorm:"column:message_type;not null;index:idx_schemas_lookup,priority:3"`
	Vendor      *string         `gorm:"column:vendor;index:idx_schemas_vendor_model,priority:1;index:idx_schemas_lookup,priority:4"`
	Model       *string         `gorm:"column:model;index:idx_schemas_vendor_model,priority:2;index:idx_schemas_lookup,priority:5"`
	Schema      json.RawMessage `gorm:"column:schema;type:jsonb;not null"`
	Status      string          `gorm:"column:status;not null;default:submitted;index:idx_schemas_status"`
	CreatedAt   time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

func (schemaEntity) TableName() string {
	return "schemas"
}

func toEntity(s *schema.Schema) *schemaEntity {
	return &schemaEntity{
		ID:          s.ID,
		OCPPVersion: string(s.OCPPVersion),
		Action:      s.Action,
		MessageType: string(s.MessageType),
		Vendor:      s.Vendor,
		Model:       s.Model,
		Schema:      s.Schema,
		Status:      string(s.Status),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func toDomain(e *schemaEntity) *schema.Schema {
	return &schema.Schema{
		ID:          e.ID,
		OCPPVersion: schema.OCPPVersion(e.OCPPVersion),
		Action:      e.Action,
		MessageType: schema.MessageType(e.MessageType),
		Vendor:      e.Vendor,
		Model:       e.Model,
		Schema:      e.Schema,
		Status:      schema.Status(e.Status),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func toDomainSlice(entities []*schemaEntity) []*schema.Schema {
	schemas := make([]*schema.Schema, len(entities))
	for i, e := range entities {
		schemas[i] = toDomain(e)
	}
	return schemas
}
