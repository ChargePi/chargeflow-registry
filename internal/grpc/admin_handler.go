package grpc

import (
	"context"
	"errors"

	adminv1 "github.com/ChargePi/chargeflow-registry/gen/proto/admin/v1"
	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	"github.com/ChargePi/chargeflow-registry/internal/pagination"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminService interface {
	List(ctx context.Context, version schema.OCPPVersion, vendor, model, action *string, msgType *schema.MessageType, status *schema.Status, limit, offset uint32) ([]*schema.Schema, int64, error)
	ChangeStatus(ctx context.Context, id uuid.UUID, status schema.Status) (*schema.Schema, error)
}

type AdminHandler struct {
	adminv1.UnimplementedAdminServiceServer
	service AdminService
}

func NewAdminHandler(service AdminService) *AdminHandler {
	return &AdminHandler{service: service}
}

func (h *AdminHandler) ListSchemas(ctx context.Context, req *adminv1.ListSchemasRequest) (*adminv1.ListSchemasResponse, error) {
	if _, ok := schemav1.SchemaStatus_name[int32(req.Status)]; !ok {
		return nil, status.Error(codes.InvalidArgument, "unknown status")
	}

	offset, err := pagination.DecodeOffset(req.PageToken)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	limit := pagination.ClampPageSize(int(req.PageSize), schema.DefaultPageSize, schema.MaxPageSize)

	var st *schema.Status
	if req.Status != schemav1.SchemaStatus_SCHEMA_STATUS_UNSPECIFIED {
		s := schemaStatusToDomain(req.Status)
		st = &s
	}

	// version, vendor, and model are left unfiltered so admins can review
	// submissions across the whole registry, not just one version/vendor/model.
	schemas, total, err := h.service.List(ctx, "", nil, nil, nil, nil, st, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &adminv1.ListSchemasResponse{
		Schemas:       schemasToProto(schemas),
		TotalSize:     total,
		NextPageToken: pagination.NextToken(offset, len(schemas), total),
	}, nil
}

func (h *AdminHandler) ChangeStatus(ctx context.Context, req *adminv1.ChangeStatusRequest) (*adminv1.ChangeStatusResponse, error) {
	id, err := uuid.Parse(req.SchemaId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid schema_id")
	}

	switch req.Status {
	case schemav1.SchemaStatus_SCHEMA_STATUS_VERIFIED, schemav1.SchemaStatus_SCHEMA_STATUS_REJECTED:
	default:
		return nil, status.Error(codes.InvalidArgument, "status must be verified or rejected")
	}

	sc, err := h.service.ChangeStatus(ctx, id, schemaStatusToDomain(req.Status))
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &adminv1.ChangeStatusResponse{Schema: schemaToProto(sc)}, nil
}
