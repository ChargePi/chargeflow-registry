package schema

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("schema.service")

type Service struct {
	repo  Repository
	cache Cache
}

func NewService(repo Repository, cache Cache) *Service {
	return &Service{repo: repo, cache: cache}
}

func (s *Service) Get(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string) (*Schema, error) {
	ctx, span := tracer.Start(ctx, "schema.Get", trace.WithAttributes(
		ocppVersionAttr(version),
		actionAttr(action),
		messageTypeAttr(msgType),
	))
	defer span.End()

	key := cacheKey(version, action, msgType, vendor, model)

	cached, err := s.cache.Get(ctx, key)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, ErrNotFound) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get schema from cache: %w", err)
	}

	sc, err := s.repo.Get(ctx, version, action, msgType, vendor, model)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get schema: %w", err)
	}

	_ = s.cache.Set(ctx, key, sc)
	return sc, nil
}

func (s *Service) Add(ctx context.Context, sc *Schema) error {
	ctx, span := tracer.Start(ctx, "schema.Add", trace.WithAttributes(
		ocppVersionAttr(sc.OCPPVersion),
		actionAttr(sc.Action),
		messageTypeAttr(sc.MessageType),
	))
	defer span.End()

	if err := s.repo.Add(ctx, sc); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("add schema: %w", err)
	}
	return nil
}

// AddPair adds request and response schemas for the same action. If the request schema is added
// successfully but the response fails, the request insertion is rolled back via deleteOne.
func (s *Service) AddPair(ctx context.Context, req, resp *Schema) error {
	ctx, span := tracer.Start(ctx, "schema.AddPair", trace.WithAttributes(
		ocppVersionAttr(req.OCPPVersion),
		actionAttr(req.Action),
	))
	defer span.End()

	if err := s.repo.Add(ctx, req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("add request schema: %w", err)
	}

	if err := s.repo.Add(ctx, resp); err != nil {
		_ = s.deleteOne(ctx, req.OCPPVersion, req.Action, req.MessageType, req.Vendor, req.Model)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("add response schema: %w", err)
	}

	return nil
}

// Delete removes both request and response schemas for an action.
// Returns ErrNotFound only if neither exists.
func (s *Service) Delete(ctx context.Context, version OCPPVersion, action string, vendor, model *string) error {
	ctx, span := tracer.Start(ctx, "schema.Delete", trace.WithAttributes(
		ocppVersionAttr(version),
		actionAttr(action),
	))
	defer span.End()

	reqErr := s.deleteOne(ctx, version, action, MessageTypeRequest, vendor, model)
	respErr := s.deleteOne(ctx, version, action, MessageTypeResponse, vendor, model)

	if errors.Is(reqErr, ErrNotFound) && errors.Is(respErr, ErrNotFound) {
		span.SetStatus(codes.Error, ErrNotFound.Error())
		return ErrNotFound
	}

	_ = s.cache.Delete(ctx, cacheKey(version, action, MessageTypeRequest, vendor, model))
	_ = s.cache.Delete(ctx, cacheKey(version, action, MessageTypeResponse, vendor, model))
	return nil
}

func (s *Service) deleteOne(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string) error {
	if err := s.repo.Delete(ctx, version, action, msgType, vendor, model); err != nil {
		return fmt.Errorf("delete schema: %w", err)
	}

	_ = s.cache.Delete(ctx, cacheKey(version, action, msgType, vendor, model))
	return nil
}

// List returns schemas matching the given filters, paginated. An empty version,
// nil vendor/model/action/status, or nil msgType is treated as "any" for that field.
func (s *Service) List(ctx context.Context, version OCPPVersion, vendor, model, action *string, msgType *MessageType, status *Status, limit, offset uint32) ([]*Schema, int64, error) {
	ctx, span := tracer.Start(ctx, "schema.List", trace.WithAttributes(
		ocppVersionAttr(version),
	))
	defer span.End()

	schemas, total, err := s.repo.List(ctx, version, vendor, model, action, msgType, status, clampPageSize(limit), offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list schemas: %w", err)
	}
	return schemas, total, nil
}

// ChangeStatus applies an admin decision to a schema, invalidating its cache entry
// so the next Get reflects the new status.
func (s *Service) ChangeStatus(ctx context.Context, id uuid.UUID, status Status) (*Schema, error) {
	ctx, span := tracer.Start(ctx, "schema.ChangeStatus")
	defer span.End()

	sc, err := s.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("change schema status: %w", err)
	}

	// Verifying doesn't change the schema's content, so a cached copy stays valid.
	// Rejection means the schema should stop being served, so evict it.
	if status == StatusRejected {
		_ = s.cache.Delete(ctx, cacheKey(sc.OCPPVersion, sc.Action, sc.MessageType, sc.Vendor, sc.Model))
	}
	return sc, nil
}

// ListVendorModels returns the OCPP version/vendor/model combinations available
// for vendor, optionally filtered to models, paginated.
func (s *Service) ListVendorModels(ctx context.Context, vendor string, models []string, limit, offset uint32) ([]*VendorModel, int64, error) {
	ctx, span := tracer.Start(ctx, "schema.ListVendorModels", trace.WithAttributes(
		vendorAttr(vendor),
	))
	defer span.End()

	vendorModels, total, err := s.repo.ListVendorModels(ctx, vendor, models, clampPageSize(limit), offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list vendor models: %w", err)
	}
	return vendorModels, total, nil
}

// ListVersions returns the content changelog for the schema with the given ID,
// newest first, paginated.
func (s *Service) ListVersions(ctx context.Context, id uuid.UUID, limit, offset uint32) ([]*SchemaVersion, int64, error) {
	ctx, span := tracer.Start(ctx, "schema.ListVersions")
	defer span.End()

	versions, total, err := s.repo.ListVersions(ctx, id, clampPageSize(limit), offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list schema versions: %w", err)
	}
	return versions, total, nil
}

// GetVersion retrieves a schema at a specific historical content version. It
// resolves the schema slot through the same cached, verified-only lookup as
// Get, then either returns that result directly (if it already is the
// requested version) or fetches the archived content from the changelog.
func (s *Service) GetVersion(ctx context.Context, version OCPPVersion, action string, msgType MessageType, vendor, model *string, schemaVersion int) (*Schema, error) {
	ctx, span := tracer.Start(ctx, "schema.GetVersion", trace.WithAttributes(
		ocppVersionAttr(version),
		actionAttr(action),
		messageTypeAttr(msgType),
	))
	defer span.End()

	sc, err := s.Get(ctx, version, action, msgType, vendor, model)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get schema: %w", err)
	}

	if sc.Version == schemaVersion {
		return sc, nil
	}

	archived, err := s.repo.GetVersion(ctx, sc.ID, schemaVersion)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get schema version: %w", err)
	}

	out := *sc
	out.Schema = archived.Schema
	out.Version = archived.Version
	out.UpdatedAt = archived.CreatedAt
	return &out, nil
}

func clampPageSize(limit uint32) uint32 {
	if limit == 0 {
		return DefaultPageSize
	}
	if limit > MaxPageSize {
		return MaxPageSize
	}
	return limit
}

func (s *Service) Upsert(ctx context.Context, sc *Schema) error {
	ctx, span := tracer.Start(ctx, "schema.Upsert", trace.WithAttributes(
		ocppVersionAttr(sc.OCPPVersion),
		actionAttr(sc.Action),
		messageTypeAttr(sc.MessageType),
	))
	defer span.End()

	if err := s.repo.Upsert(ctx, sc); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert schema: %w", err)
	}

	_ = s.cache.Delete(ctx, cacheKey(sc.OCPPVersion, sc.Action, sc.MessageType, sc.Vendor, sc.Model))
	return nil
}

func cacheKey(version OCPPVersion, action string, msgType MessageType, vendor, model *string) string {
	v, m := "nil", "nil"
	if vendor != nil {
		v = *vendor
	}
	if model != nil {
		m = *model
	}
	return fmt.Sprintf("schema:%s:%s:%s:%s:%s", version, action, msgType, v, m)
}
