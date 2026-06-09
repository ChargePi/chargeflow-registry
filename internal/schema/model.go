package schema

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type OCPPVersion string

const (
	OCPPVersion16  OCPPVersion = "1.6"
	OCPPVersion201 OCPPVersion = "2.0.1"
	OCPPVersion21  OCPPVersion = "2.1"
)

type MessageType string

const (
	MessageTypeRequest  MessageType = "request"
	MessageTypeResponse MessageType = "response"
)

type Schema struct {
	ID          uuid.UUID
	OCPPVersion OCPPVersion
	Action      string
	MessageType MessageType
	Vendor      *string
	Model       *string
	Schema      json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
