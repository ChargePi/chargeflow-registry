package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/samber/lo"
)

const (
	schemaURITemplate  = "schema://{version}/{action}/{type}{?vendor,model}"
	schemasURITemplate = "schemas://{version}{?vendor,model,action,message_type}"
)

func registerResources(s *server.MCPServer, h *handlers) {
	s.AddResourceTemplate(
		mcplib.NewResourceTemplate(
			schemaURITemplate,
			"OCPP Schema",
			mcplib.WithTemplateDescription("A registered OCPP JSON schema identified by version, action, and message type, optionally scoped to a vendor and model. Example: schema://1.6/Authorize/request?vendor=ABB&model=Terra54"),
			mcplib.WithTemplateMIMEType("application/json"),
		),
		h.readSchema,
	)

	s.AddResourceTemplate(
		mcplib.NewResourceTemplate(
			schemasURITemplate,
			"OCPP Schemas List",
			mcplib.WithTemplateDescription(`All registered OCPP schemas for a given OCPP version, optionally filtered by vendor, model, action, and message_type ("request" or "response"). Example: schemas://1.6?vendor=ABB&action=Authorize&message_type=request`),
			mcplib.WithTemplateMIMEType("application/json"),
		),
		h.listSchemas,
	)
}

// readSchema handles schema://{version}/{action}/{type}{?vendor,model}
// URI example: schema://1.6/Authorize/request?vendor=ABB&model=Terra54
func (h *handlers) readSchema(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	version, action, msgType, filter, err := parseSchemaURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	sc, err := h.schemaSvc.Get(ctx, schema.OCPPVersion(version), action, schema.MessageType(msgType), filter.Vendor, filter.Model)
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return nil, fmt.Errorf("schema not found: %s", req.Params.URI)
		}
		return nil, fmt.Errorf("get schema: %w", err)
	}

	out, err := json.Marshal(sc)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return []mcplib.ResourceContents{
		mcplib.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(out),
		},
	}, nil
}

// listSchemas handles schemas://{version}{?vendor,model,action,message_type}
// URI example: schemas://1.6?vendor=ABB&model=Terra54&action=Authorize&message_type=request
func (h *handlers) listSchemas(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	version, filter, err := parseSchemasURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	// The MCP API is user-facing, so only admin-verified schemas are ever listed here.
	schemas, _, err := h.schemaSvc.List(ctx, schema.OCPPVersion(version), filter.Vendor, filter.Model, filter.Action, filter.MessageType, lo.ToPtr(schema.StatusVerified), schema.MaxPageSize, 0)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	out, err := json.Marshal(schemas)
	if err != nil {
		return nil, fmt.Errorf("marshal schemas: %w", err)
	}

	return []mcplib.ResourceContents{
		mcplib.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(out),
		},
	}, nil
}

// schemaFilter holds the optional query-string filters shared by the schema:// and schemas:// resource templates.
type schemaFilter struct {
	Vendor      *string
	Model       *string
	Action      *string
	MessageType *schema.MessageType
}

// parseSchemaURI parses schema://{version}/{action}/{type}{?vendor,model} into its components.
// Example: schema://1.6/Authorize/request?vendor=ABB&model=Terra54 → ("1.6", "Authorize", "request", {Vendor: "ABB", Model: "Terra54"})
func parseSchemaURI(uri string) (version, action, msgType string, filter schemaFilter, err error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "schema" {
		return "", "", "", schemaFilter{}, fmt.Errorf("invalid schema URI %q: expected schema://{version}/{action}/{type}", uri)
	}

	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if u.Host == "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", schemaFilter{}, fmt.Errorf("invalid schema URI %q: expected schema://{version}/{action}/{type}", uri)
	}

	return u.Host, parts[0], parts[1], parseSchemaFilter(u.RawQuery), nil
}

// parseSchemasURI parses schemas://{version}{?vendor,model,action,message_type} and returns the version and optional filters.
// Example: schemas://1.6?vendor=ABB&action=Authorize&message_type=request → ("1.6", {Vendor: "ABB", Action: "Authorize", MessageType: "request"})
func parseSchemasURI(uri string) (version string, filter schemaFilter, err error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "schemas" || u.Host == "" {
		return "", schemaFilter{}, fmt.Errorf("invalid schemas URI %q: expected schemas://{version}", uri)
	}

	return u.Host, parseSchemaFilter(u.RawQuery), nil
}

// parseSchemaFilter extracts the optional vendor, model, action, and message_type filters from a URI's raw query string.
func parseSchemaFilter(rawQuery string) schemaFilter {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return schemaFilter{}
	}

	var filter schemaFilter
	if v := values.Get("vendor"); v != "" {
		filter.Vendor = &v
	}
	if m := values.Get("model"); m != "" {
		filter.Model = &m
	}
	if a := values.Get("action"); a != "" {
		filter.Action = &a
	}
	if mt := values.Get("message_type"); mt != "" {
		msgType := schema.MessageType(mt)
		filter.MessageType = &msgType
	}
	return filter
}
