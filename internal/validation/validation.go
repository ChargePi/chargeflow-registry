package validation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/ChargePi/chargeflow/pkg/ocpp"
	cfvalidator "github.com/ChargePi/chargeflow/pkg/validator"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("validation.service")

type Violation struct {
	Field   string
	Message string
}

type Result struct {
	Valid      bool
	Violations []Violation
}

type Request struct {
	Version     schema.OCPPVersion
	Action      string
	MessageType schema.MessageType
	Vendor      *string
	Model       *string
	Message     []byte
}

// MaxBulkMessages caps the number of messages accepted by a single
// BulkValidateMessages call.
const MaxBulkMessages = 200

// BulkRequest validates multiple OCPP-J messages against the given OCPP
// version, vendor and model. Each message carries its own action and
// message type, so unlike Request these aren't supplied once for the batch.
type BulkRequest struct {
	Version  schema.OCPPVersion
	Vendor   *string
	Model    *string
	Messages []string
}

// ReportEntry is one message's validation outcome within a bulk report,
// tagged with its position in the request's Messages list.
type ReportEntry struct {
	Index      int
	Valid      bool
	Violations []Violation
}

type SchemaGetter interface {
	Get(ctx context.Context, version schema.OCPPVersion, action string, msgType schema.MessageType, vendor, model *string) (*schema.Schema, error)
}

type Service struct {
	validator *cfvalidator.Validator
}

func NewService(schemas SchemaGetter, logger *zap.Logger) *Service {
	return &Service{
		validator: cfvalidator.NewValidator(logger, newRegistryAdapter(schemas)),
	}
}

func (s *Service) ValidateMessage(ctx context.Context, req Request) (*Result, error) {
	ctx, span := tracer.Start(ctx, "validation.ValidateMessage", trace.WithAttributes(
		attribute.String("ocpp.version", string(req.Version)),
		attribute.String("ocpp.action", req.Action),
		attribute.String("ocpp.message_type", string(req.MessageType)),
	))
	defer span.End()

	var payload any
	if err := json.Unmarshal(req.Message, &payload); err != nil {
		return &Result{
			Valid:      false,
			Violations: []Violation{{Field: "/", Message: "invalid JSON: " + err.Error()}},
		}, nil
	}

	msg, err := buildOCPPMessage(req.Action, req.MessageType, payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	octx := ocpp.OcppContext{
		Version: ocpp.Version(req.Version),
		Vendor:  derefString(req.Vendor),
		Model:   derefString(req.Model),
	}

	result, err := s.validator.ValidateMessage(octx, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("validate message: %w", err)
	}

	violations := make([]Violation, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		violations = append(violations, Violation{Message: e})
	}

	span.SetAttributes(attribute.Bool("validation.valid", result.IsValid()))
	return &Result{Valid: result.IsValid(), Violations: violations}, nil
}

// bulkEnvelope is the JSON shape of one message in a BulkRequest: an OCPP-J
// frame with messageTypeId, action and payload as named object fields
// (rather than the positional array used on the wire), so each message's
// action and message type can be read off the message itself.
type bulkEnvelope struct {
	MessageTypeId ocpp.MessageType `json:"messageTypeId"`
	Action        string           `json:"action"`
	Payload       json.RawMessage  `json:"payload"`
}

// ValidateMessages validates each of req.Messages, deriving the action and
// message type from the message itself, and returns one report entry per
// message in input order.
func (s *Service) ValidateMessages(ctx context.Context, req BulkRequest) ([]ReportEntry, error) {
	ctx, span := tracer.Start(ctx, "validation.ValidateMessages", trace.WithAttributes(
		attribute.String("ocpp.version", string(req.Version)),
		attribute.Int("ocpp.message_count", len(req.Messages)),
	))
	defer span.End()

	entries := make([]ReportEntry, len(req.Messages))
	for i, msg := range req.Messages {
		var env bulkEnvelope
		if err := json.Unmarshal([]byte(msg), &env); err != nil {
			entries[i] = ReportEntry{Index: i, Violations: []Violation{{Field: "/", Message: "invalid JSON: " + err.Error()}}}
			continue
		}

		msgType, ok := messageTypeFromOCPP(env.MessageTypeId)
		if !ok {
			entries[i] = ReportEntry{Index: i, Violations: []Violation{{Field: "/messageTypeId", Message: fmt.Sprintf("unsupported message type: %d", env.MessageTypeId)}}}
			continue
		}
		if env.Action == "" {
			entries[i] = ReportEntry{Index: i, Violations: []Violation{{Field: "/action", Message: "action is required"}}}
			continue
		}

		result, err := s.ValidateMessage(ctx, Request{
			Version:     req.Version,
			Action:      env.Action,
			MessageType: msgType,
			Vendor:      req.Vendor,
			Model:       req.Model,
			Message:     env.Payload,
		})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("validate message %d: %w", i, err)
		}
		entries[i] = ReportEntry{Index: i, Valid: result.Valid, Violations: result.Violations}
	}

	return entries, nil
}

func messageTypeFromOCPP(id ocpp.MessageType) (schema.MessageType, bool) {
	switch id {
	case ocpp.CALL:
		return schema.MessageTypeRequest, true
	case ocpp.CALL_RESULT:
		return schema.MessageTypeResponse, true
	default:
		return "", false
	}
}

func buildOCPPMessage(action string, msgType schema.MessageType, payload any) (ocpp.Message, error) {
	switch msgType {
	case schema.MessageTypeRequest:
		return &ocpp.Call{
			MessageTypeId: ocpp.CALL,
			UniqueId:      "validation",
			Action:        action,
			Payload:       payload,
		}, nil
	case schema.MessageTypeResponse:
		return &ocpp.CallResult{
			MessageTypeId: ocpp.CALL_RESULT,
			UniqueId:      "validation",
			Action:        action,
			Payload:       payload,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msgType)
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
