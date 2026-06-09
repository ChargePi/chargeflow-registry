package validation

import (
	"context"
	"strings"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/ChargePi/chargeflow/pkg/schema_registry"
	"github.com/kaptinlin/jsonschema"
	"github.com/samber/lo"
)

var _ schema_registry.SchemaRegistry = (*registryAdapter)(nil)

// registryAdapter bridges SchemaGetter to the chargeflow SchemaRegistry interface.
// Vendor/model arrive via GetSchemaRequest.OcppContext, so the adapter is stateless
// and can be created once rather than per request.
// Actions arrive in chargeflow's "ActionRequest"/"ActionResponse" form; the suffix
// is stripped to look up the base action in our storage.
type registryAdapter struct {
	getter   SchemaGetter
	compiler *jsonschema.Compiler
}

func newRegistryAdapter(getter SchemaGetter) *registryAdapter {
	return &registryAdapter{
		getter:   getter,
		compiler: jsonschema.NewCompiler(),
	}
}

func (a *registryAdapter) GetSchema(ctx context.Context, req schema_registry.GetSchemaRequest) (*jsonschema.Schema, bool) {
	var msgType schema.MessageType
	var baseAction string

	switch {
	case strings.HasSuffix(req.Action, "Request"):
		msgType = schema.MessageTypeRequest
		baseAction = strings.TrimSuffix(req.Action, "Request")
	case strings.HasSuffix(req.Action, "Response"):
		msgType = schema.MessageTypeResponse
		baseAction = strings.TrimSuffix(req.Action, "Response")
	default:
		return nil, false
	}

	s, err := a.getter.Get(ctx, schema.OCPPVersion(req.OcppContext.Version), baseAction, msgType, lo.EmptyableToPtr(req.OcppContext.Vendor), lo.EmptyableToPtr(req.OcppContext.Model))
	if err != nil {
		return nil, false
	}

	compiled, err := a.compiler.Compile(s.Schema)
	if err != nil {
		return nil, false
	}

	return compiled, true
}

func (a *registryAdapter) RegisterSchema(_ context.Context, _ schema_registry.CreateSchemaRequest) error {
	return nil
}

func (a *registryAdapter) DeleteSchema(_ context.Context, _ schema_registry.DeleteSchemaRequest) error {
	return nil
}

func (a *registryAdapter) Type() string {
	return "remote"
}
