package grpc

import (
	"context"
	"errors"

	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SchemaService interface {
	AddPair(ctx context.Context, req, resp *schema.Schema) error
	Upsert(ctx context.Context, s *schema.Schema) error
	Delete(ctx context.Context, version schema.OCPPVersion, action string, vendor, model *string) error
	ListVendorModels(ctx context.Context, vendor string, models []string) ([]*schema.VendorModel, error)
}

type Handler struct {
	schemav1.UnimplementedSchemaRegistryServiceServer
	service SchemaService
}

func NewHandler(service SchemaService) *Handler {
	return &Handler{service: service}
}

// validateVendorModel enforces that model is never set without vendor.
// A vendor-only entry is valid and typically represents a CSMS vendor rather than a hardware manufacturer.
func validateVendorModel(vendor, model *string) error {
	if model != nil && vendor == nil {
		return status.Error(codes.InvalidArgument, "vendor is required when model is specified")
	}
	return nil
}

func validateActionKey(version schemav1.OcppVersion, action string) error {
	if version == schemav1.OcppVersion_OCPP_VERSION_UNSPECIFIED {
		return status.Error(codes.InvalidArgument, "ocpp_version is required")
	}
	if _, ok := schemav1.OcppVersion_name[int32(version)]; !ok {
		return status.Error(codes.InvalidArgument, "unknown ocpp_version")
	}
	if action == "" {
		return status.Error(codes.InvalidArgument, "action is required")
	}
	return nil
}

func validateSchemaKey(version schemav1.OcppVersion, action string, msgType schemav1.MessageType) error {
	if err := validateActionKey(version, action); err != nil {
		return err
	}
	if msgType == schemav1.MessageType_MESSAGE_TYPE_UNSPECIFIED {
		return status.Error(codes.InvalidArgument, "message_type is required")
	}
	if _, ok := schemav1.MessageType_name[int32(msgType)]; !ok {
		return status.Error(codes.InvalidArgument, "unknown message_type")
	}
	return nil
}

func (h *Handler) AddSchema(ctx context.Context, req *schemav1.AddSchemaRequest) (*schemav1.AddSchemaResponse, error) {
	if err := validateActionKey(req.OcppVersion, req.Action); err != nil {
		return nil, err
	}
	if err := validateVendorModel(req.Vendor, req.Model); err != nil {
		return nil, err
	}
	if len(req.RequestSchema) == 0 {
		return nil, status.Error(codes.InvalidArgument, "request_schema is required")
	}
	if len(req.ResponseSchema) == 0 {
		return nil, status.Error(codes.InvalidArgument, "response_schema is required")
	}

	base := schema.Schema{
		OCPPVersion: ocppVersionToDomain(req.OcppVersion),
		Action:      req.Action,
		Vendor:      req.Vendor,
		Model:       req.Model,
		Status:      schema.StatusSubmitted,
	}
	reqSchema := base
	reqSchema.MessageType = schema.MessageTypeRequest
	reqSchema.Schema = req.RequestSchema

	respSchema := base
	respSchema.MessageType = schema.MessageTypeResponse
	respSchema.Schema = req.ResponseSchema

	if err := h.service.AddPair(ctx, &reqSchema, &respSchema); err != nil {
		if errors.Is(err, schema.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemav1.AddSchemaResponse{}, nil
}

func (h *Handler) UpsertSchema(ctx context.Context, req *schemav1.UpsertSchemaRequest) (*schemav1.UpsertSchemaResponse, error) {
	if err := validateSchemaKey(req.OcppVersion, req.Action, req.MessageType); err != nil {
		return nil, err
	}
	if err := validateVendorModel(req.Vendor, req.Model); err != nil {
		return nil, err
	}
	if len(req.Schema) == 0 {
		return nil, status.Error(codes.InvalidArgument, "schema is required")
	}

	s := &schema.Schema{
		OCPPVersion: ocppVersionToDomain(req.OcppVersion),
		Action:      req.Action,
		MessageType: messageTypeToDomain(req.MessageType),
		Vendor:      req.Vendor,
		Model:       req.Model,
		Schema:      req.Schema,
		Status:      schema.StatusSubmitted,
	}

	if err := h.service.Upsert(ctx, s); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemav1.UpsertSchemaResponse{}, nil
}

func (h *Handler) DeleteSchema(ctx context.Context, req *schemav1.DeleteSchemaRequest) (*schemav1.DeleteSchemaResponse, error) {
	if err := validateActionKey(req.OcppVersion, req.Action); err != nil {
		return nil, err
	}
	if err := validateVendorModel(req.Vendor, req.Model); err != nil {
		return nil, err
	}

	err := h.service.Delete(ctx, ocppVersionToDomain(req.OcppVersion), req.Action, req.Vendor, req.Model)
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemav1.DeleteSchemaResponse{}, nil
}

func (h *Handler) ListVendorModels(ctx context.Context, req *schemav1.ListVendorModelsRequest) (*schemav1.ListVendorModelsResponse, error) {
	if req.Vendor == "" {
		return nil, status.Error(codes.InvalidArgument, "vendor is required")
	}

	vendorModels, err := h.service.ListVendorModels(ctx, req.Vendor, req.Models)
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "no models found for vendor")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemav1.ListVendorModelsResponse{VendorModels: vendorModelsToProto(vendorModels)}, nil
}
