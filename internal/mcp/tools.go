package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/ChargePi/chargeflow-registry/internal/validation"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/samber/lo"
)

func registerTools(s *server.MCPServer, h *handlers) {
	s.AddTool(newSchemaValidationTool(), h.schemaValidation)
	s.AddTool(newRegisterSchemaTool(), h.registerSchema)
	s.AddTool(newRemoveSchemaTool(), h.removeSchema)
	s.AddTool(newGetSchemaTool(), h.getSchema)
	s.AddTool(newQuerySchemasTool(), h.querySchemas)
}

func newSchemaValidationTool() mcplib.Tool {
	return mcplib.NewTool("schema_validation",
		mcplib.WithDescription("Validate an OCPP message payload against its registered JSON schema"),
		mcplib.WithString("ocpp_version",
			mcplib.Required(),
			mcplib.Description(`OCPP version: "1.6", "2.0.1", or "2.1"`),
			mcplib.Enum("1.6", "2.0.1", "2.1"),
		),
		mcplib.WithString("action",
			mcplib.Required(),
			mcplib.Description("OCPP action name (e.g. Authorize, BootNotification)"),
		),
		mcplib.WithString("message_type",
			mcplib.Required(),
			mcplib.Description(`Message direction: "request" or "response"`),
			mcplib.Enum("request", "response"),
		),
		mcplib.WithString("message",
			mcplib.Required(),
			mcplib.Description("JSON-encoded OCPP message payload to validate"),
		),
		mcplib.WithString("vendor",
			mcplib.Description("Optional EV manufacturer identifier for vendor-specific schema lookup"),
		),
		mcplib.WithString("model",
			mcplib.Description("Optional EV model identifier (requires vendor)"),
		),
	)
}

func newRegisterSchemaTool() mcplib.Tool {
	return mcplib.NewTool("register_schema",
		mcplib.WithDescription("Register request and response JSON schemas for an OCPP action"),
		mcplib.WithString("ocpp_version",
			mcplib.Required(),
			mcplib.Description(`OCPP version: "1.6", "2.0.1", or "2.1"`),
			mcplib.Enum("1.6", "2.0.1", "2.1"),
		),
		mcplib.WithString("action",
			mcplib.Required(),
			mcplib.Description("OCPP action name (e.g. Authorize, BootNotification)"),
		),
		mcplib.WithString("request_schema",
			mcplib.Required(),
			mcplib.Description("JSON Schema definition for the request message"),
		),
		mcplib.WithString("response_schema",
			mcplib.Required(),
			mcplib.Description("JSON Schema definition for the response message"),
		),
		mcplib.WithString("vendor",
			mcplib.Description("Optional EV manufacturer identifier for vendor-specific schema"),
		),
		mcplib.WithString("model",
			mcplib.Description("Optional EV model identifier (requires vendor)"),
		),
	)
}

func newRemoveSchemaTool() mcplib.Tool {
	return mcplib.NewTool("remove_schema",
		mcplib.WithDescription("Remove request and response schemas for an OCPP action"),
		mcplib.WithString("ocpp_version",
			mcplib.Required(),
			mcplib.Description(`OCPP version: "1.6", "2.0.1", or "2.1"`),
			mcplib.Enum("1.6", "2.0.1", "2.1"),
		),
		mcplib.WithString("action",
			mcplib.Required(),
			mcplib.Description("OCPP action name"),
		),
		mcplib.WithString("vendor",
			mcplib.Description("Optional EV manufacturer identifier"),
		),
		mcplib.WithString("model",
			mcplib.Description("Optional EV model identifier (requires vendor)"),
		),
	)
}

func newGetSchemaTool() mcplib.Tool {
	return mcplib.NewTool("get_schema",
		mcplib.WithDescription("Retrieve a specific OCPP JSON schema by version, action, and message type"),
		mcplib.WithString("ocpp_version",
			mcplib.Required(),
			mcplib.Description(`OCPP version: "1.6", "2.0.1", or "2.1"`),
			mcplib.Enum("1.6", "2.0.1", "2.1"),
		),
		mcplib.WithString("action",
			mcplib.Required(),
			mcplib.Description("OCPP action name"),
		),
		mcplib.WithString("message_type",
			mcplib.Required(),
			mcplib.Description(`Message direction: "request" or "response"`),
			mcplib.Enum("request", "response"),
		),
		mcplib.WithString("vendor",
			mcplib.Description("Optional EV manufacturer identifier for vendor-specific schema lookup"),
		),
		mcplib.WithString("model",
			mcplib.Description("Optional EV model identifier (requires vendor)"),
		),
	)
}

func newQuerySchemasTool() mcplib.Tool {
	return mcplib.NewTool("query_schemas",
		mcplib.WithDescription("List all registered OCPP schemas for a given version, optionally filtered by vendor and model"),
		mcplib.WithString("ocpp_version",
			mcplib.Required(),
			mcplib.Description(`OCPP version to query: "1.6", "2.0.1", or "2.1"`),
			mcplib.Enum("1.6", "2.0.1", "2.1"),
		),
		mcplib.WithString("vendor",
			mcplib.Description("Optional EV manufacturer identifier filter"),
		),
		mcplib.WithString("model",
			mcplib.Description("Optional EV model identifier filter"),
		),
	)
}

func (h *handlers) schemaValidation(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	versionStr := req.GetString("ocpp_version", "")
	action := req.GetString("action", "")
	msgTypeStr := req.GetString("message_type", "")
	message := req.GetString("message", "")
	vendor := optionalString(req.GetArguments()["vendor"])
	model := optionalString(req.GetArguments()["model"])

	result, err := h.validationSvc.ValidateMessage(ctx, validation.Request{
		Version:     schema.OCPPVersion(versionStr),
		Action:      action,
		MessageType: schema.MessageType(msgTypeStr),
		Vendor:      vendor,
		Model:       model,
		Message:     []byte(message),
	})
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return mcplib.NewToolResultError("schema not found for the given parameters"), nil
		}
		return mcplib.NewToolResultError(fmt.Sprintf("validation error: %s", err)), nil
	}

	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	return mcplib.NewToolResultText(string(out)), nil
}

func (h *handlers) registerSchema(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	versionStr := req.GetString("ocpp_version", "")
	action := req.GetString("action", "")
	requestSchemaStr := req.GetString("request_schema", "")
	responseSchemaStr := req.GetString("response_schema", "")
	vendor := optionalString(req.GetArguments()["vendor"])
	model := optionalString(req.GetArguments()["model"])

	reqSchema := &schema.Schema{
		OCPPVersion: schema.OCPPVersion(versionStr),
		Action:      action,
		MessageType: schema.MessageTypeRequest,
		Vendor:      vendor,
		Model:       model,
		Schema:      json.RawMessage(requestSchemaStr),
	}
	respSchema := &schema.Schema{
		OCPPVersion: schema.OCPPVersion(versionStr),
		Action:      action,
		MessageType: schema.MessageTypeResponse,
		Vendor:      vendor,
		Model:       model,
		Schema:      json.RawMessage(responseSchemaStr),
	}

	if err := h.schemaSvc.AddPair(ctx, reqSchema, respSchema); err != nil {
		if errors.Is(err, schema.ErrAlreadyExists) {
			return mcplib.NewToolResultError("schema already exists for this action and version"), nil
		}
		return mcplib.NewToolResultError(fmt.Sprintf("failed to register schema: %s", err)), nil
	}

	return mcplib.NewToolResultText("schema registered successfully"), nil
}

func (h *handlers) removeSchema(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	versionStr := req.GetString("ocpp_version", "")
	action := req.GetString("action", "")
	vendor := optionalString(req.GetArguments()["vendor"])
	model := optionalString(req.GetArguments()["model"])

	if err := h.schemaSvc.Delete(ctx, schema.OCPPVersion(versionStr), action, vendor, model); err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return mcplib.NewToolResultError("schema not found"), nil
		}
		return mcplib.NewToolResultError(fmt.Sprintf("failed to remove schema: %s", err)), nil
	}

	return mcplib.NewToolResultText("schema removed successfully"), nil
}

func (h *handlers) getSchema(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	versionStr := req.GetString("ocpp_version", "")
	action := req.GetString("action", "")
	msgTypeStr := req.GetString("message_type", "")
	vendor := optionalString(req.GetArguments()["vendor"])
	model := optionalString(req.GetArguments()["model"])

	sc, err := h.schemaSvc.Get(ctx,
		schema.OCPPVersion(versionStr),
		action,
		schema.MessageType(msgTypeStr),
		vendor, model,
	)
	if err != nil {
		if errors.Is(err, schema.ErrNotFound) {
			return mcplib.NewToolResultError("schema not found"), nil
		}
		return mcplib.NewToolResultError(fmt.Sprintf("failed to get schema: %s", err)), nil
	}

	out, err := json.Marshal(sc)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return mcplib.NewToolResultText(string(out)), nil
}

func (h *handlers) querySchemas(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	versionStr := req.GetString("ocpp_version", "")
	vendor := optionalString(req.GetArguments()["vendor"])
	model := optionalString(req.GetArguments()["model"])

	// The MCP API is user-facing, so only admin-verified schemas are ever listed here.
	schemas, _, err := h.schemaSvc.List(ctx, schema.OCPPVersion(versionStr), vendor, model, nil, nil, lo.ToPtr(schema.StatusVerified), schema.MaxPageSize, 0)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to query schemas: %s", err)), nil
	}

	out, err := json.Marshal(schemas)
	if err != nil {
		return nil, fmt.Errorf("marshal schemas: %w", err)
	}

	return mcplib.NewToolResultText(string(out)), nil
}

func optionalString(v any) *string {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	return &s
}
