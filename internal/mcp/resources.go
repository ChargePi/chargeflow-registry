package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/samber/lo"
)

const (
	schemaURITemplate  = "schema://{version}/{action}/{type}"
	schemasURITemplate = "schemas://{version}"
)

func registerResources(s *server.MCPServer, h *handlers) {
	s.AddResourceTemplate(
		mcplib.NewResourceTemplate(
			schemaURITemplate,
			"OCPP Schema",
			mcplib.WithTemplateDescription("A registered OCPP JSON schema identified by version, action, and message type. Example: schema://1.6/Authorize/request"),
			mcplib.WithTemplateMIMEType("application/json"),
		),
		h.readSchema,
	)

	s.AddResourceTemplate(
		mcplib.NewResourceTemplate(
			schemasURITemplate,
			"OCPP Schemas List",
			mcplib.WithTemplateDescription("All registered OCPP schemas for a given OCPP version. Example: schemas://1.6"),
			mcplib.WithTemplateMIMEType("application/json"),
		),
		h.listSchemas,
	)
}

// readSchema handles schema://{version}/{action}/{type}
// URI example: schema://1.6/Authorize/request
func (h *handlers) readSchema(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	version, action, msgType, err := parseSchemaURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	sc, err := h.schemaSvc.Get(ctx, schema.OCPPVersion(version), action, schema.MessageType(msgType), nil, nil)
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

// listSchemas handles schemas://{version}
// URI example: schemas://1.6
func (h *handlers) listSchemas(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	version, err := parseSchemasURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	// The MCP API is user-facing, so only admin-verified schemas are ever listed here.
	schemas, _, err := h.schemaSvc.List(ctx, schema.OCPPVersion(version), nil, nil, nil, nil, lo.ToPtr(schema.StatusVerified), schema.MaxPageSize, 0)
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

// parseSchemaURI parses schema://{version}/{action}/{type} into its components.
// Example: schema://1.6/Authorize/request → ("1.6", "Authorize", "request")
func parseSchemaURI(uri string) (version, action, msgType string, err error) {
	path := strings.TrimPrefix(uri, "schema://")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid schema URI %q: expected schema://{version}/{action}/{type}", uri)
	}
	return parts[0], parts[1], parts[2], nil
}

// parseSchemasURI parses schemas://{version} and returns the version.
// Example: schemas://1.6 → "1.6"
func parseSchemasURI(uri string) (string, error) {
	version := strings.TrimPrefix(uri, "schemas://")
	if version == "" || version == uri {
		return "", fmt.Errorf("invalid schemas URI %q: expected schemas://{version}", uri)
	}
	return version, nil
}
