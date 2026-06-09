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
