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

	resp := &schemav1.ValidateMessageResponse{Valid: result.Valid}
	for _, v := range result.Violations {
		resp.Violations = append(resp.Violations, &schemav1.ValidationViolation{
			Field:   v.Field,
			Message: v.Message,
		})
	}

	return resp, nil
}
