package grpc

import (
	"context"
	"errors"

	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/ChargePi/chargeflow-registry/internal/validation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ValidationService interface {
	ValidateMessage(ctx context.Context, req validation.Request) (*validation.Result, error)
	ValidateMessages(ctx context.Context, req validation.BulkRequest) ([]validation.ReportEntry, error)
}

type ValidationHandler struct {
	schemav1.UnimplementedSchemaValidationServiceServer
	service ValidationService
}

func NewValidationHandler(service ValidationService) *ValidationHandler {
	return &ValidationHandler{service: service}
}

func (h *ValidationHandler) ValidateMessage(ctx context.Context, req *schemav1.ValidateMessageRequest) (*schemav1.ValidateMessageResponse, error) {
	if err := validateSchemaKey(req.OcppVersion, req.Action, req.MessageType); err != nil {
		return nil, err
	}
	if err := validateVendorModel(req.Vendor, req.Model); err != nil {
		return nil, err
	}
	if len(req.Message) == 0 {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	result, err := h.service.ValidateMessage(ctx, validation.Request{
		Version:     ocppVersionToDomain(req.OcppVersion),
		Action:      req.Action,
		MessageType: messageTypeToDomain(req.MessageType),
		Vendor:      req.Vendor,
		Model:       req.Model,
		Message:     req.Message,
	})
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "no schema found for the given parameters")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemav1.ValidateMessageResponse{Valid: result.Valid, Violations: violationsToProto(result.Violations)}, nil
}

// BulkValidateMessages validates a list of OCPP-J messages against the given
// OCPP version, vendor and model, reading each message's own action and
// message type off it, and returns one report entry per message, in the
// same order they were supplied.
func (h *ValidationHandler) BulkValidateMessages(ctx context.Context, req *schemav1.BulkValidateMessagesRequest) (*schemav1.BulkValidateMessagesResponse, error) {
	if err := validateOcppVersion(req.OcppVersion); err != nil {
		return nil, err
	}
	if err := validateVendorModel(req.Vendor, req.Model); err != nil {
		return nil, err
	}
	if len(req.Messages) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one message is required")
	}
	if len(req.Messages) > validation.MaxBulkMessages {
		return nil, status.Errorf(codes.InvalidArgument, "too many messages: max %d per request", validation.MaxBulkMessages)
	}

	entries, err := h.service.ValidateMessages(ctx, validation.BulkRequest{
		Version:  ocppVersionToDomain(req.OcppVersion),
		Vendor:   req.Vendor,
		Model:    req.Model,
		Messages: req.Messages,
	})
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "no schema found for the given parameters")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	results := make([]*schemav1.ValidationReportEntry, len(entries))
	for i, e := range entries {
		results[i] = &schemav1.ValidationReportEntry{
			Index:      int32(e.Index),
			Valid:      e.Valid,
			Violations: violationsToProto(e.Violations),
		}
	}

	return &schemav1.BulkValidateMessagesResponse{Results: results}, nil
}

func violationsToProto(violations []validation.Violation) []*schemav1.ValidationViolation {
	out := make([]*schemav1.ValidationViolation, len(violations))
	for i, v := range violations {
		out[i] = &schemav1.ValidationViolation{Field: v.Field, Message: v.Message}
	}
	return out
}
