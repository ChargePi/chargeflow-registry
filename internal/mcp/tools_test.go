package mcp

import (
	"context"
	"encoding/json"
	"testing"

	schemamocks "github.com/ChargePi/chargeflow-registry/gen/mocks/mcp"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/google/uuid"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/mock"
)

func TestParseToolFilter(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want schemaFilter
	}{
		{
			name: "empty args",
			args: map[string]any{},
			want: schemaFilter{},
		},
		{
			name: "vendor and model",
			args: map[string]any{"vendor": "ABB", "model": "Terra54"},
			want: schemaFilter{Vendor: strPtr("ABB"), Model: strPtr("Terra54")},
		},
		{
			name: "all filters",
			args: map[string]any{"vendor": "ABB", "model": "Terra54", "action": "Authorize", "message_type": "request"},
			want: schemaFilter{
				Vendor:      strPtr("ABB"),
				Model:       strPtr("Terra54"),
				Action:      strPtr("Authorize"),
				MessageType: msgTypePtr("request"),
			},
		},
		{
			name: "non-string values ignored",
			args: map[string]any{"vendor": 42, "model": nil},
			want: schemaFilter{},
		},
		{
			name: "empty strings ignored",
			args: map[string]any{"vendor": "", "model": ""},
			want: schemaFilter{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertFilterEq(t, parseToolFilter(tt.args), tt.want)
		})
	}
}

func toolRequest(args map[string]any) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: args}}
}

func TestGetSchemaVersionArg(t *testing.T) {
	action := "BootNotification"
	current := &schema.Schema{Action: action, Version: 2, Schema: json.RawMessage(`{"v":2}`)}

	t.Run("omitted version uses the latest, cached Get path", func(t *testing.T) {
		reg := schemamocks.NewMockSchemaRegistry(t)
		h := &handlers{schemaSvc: reg}

		reg.EXPECT().
			Get(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
			Return(current, nil)

		res, err := h.getSchema(context.Background(), toolRequest(map[string]any{
			"ocpp_version": "1.6", "action": action, "message_type": "request",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("got error result: %v", res.Content)
		}
	})

	t.Run("a specific version calls GetVersion instead", func(t *testing.T) {
		reg := schemamocks.NewMockSchemaRegistry(t)
		h := &handlers{schemaSvc: reg}

		archived := &schema.Schema{Action: action, Version: 1, Schema: json.RawMessage(`{"v":1}`)}
		reg.EXPECT().
			GetVersion(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil), 1).
			Return(archived, nil)

		res, err := h.getSchema(context.Background(), toolRequest(map[string]any{
			"ocpp_version": "1.6", "action": action, "message_type": "request", "version": float64(1),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("got error result: %v", res.Content)
		}
	})

	t.Run("a nonexistent version reports not found", func(t *testing.T) {
		reg := schemamocks.NewMockSchemaRegistry(t)
		h := &handlers{schemaSvc: reg}

		reg.EXPECT().
			GetVersion(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil), 99).
			Return(nil, schema.ErrNotFound)

		res, err := h.getSchema(context.Background(), toolRequest(map[string]any{
			"ocpp_version": "1.6", "action": action, "message_type": "request", "version": float64(99),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("got a success result, want a not-found error")
		}
	})
}

func TestListSchemaVersionsTool(t *testing.T) {
	action := "BootNotification"

	t.Run("schema not found", func(t *testing.T) {
		reg := schemamocks.NewMockSchemaRegistry(t)
		h := &handlers{schemaSvc: reg}

		reg.EXPECT().
			Get(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
			Return(nil, schema.ErrNotFound)

		res, err := h.listSchemaVersions(context.Background(), toolRequest(map[string]any{
			"ocpp_version": "1.6", "action": action, "message_type": "request",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("got a success result, want a not-found error")
		}
	})

	t.Run("resolves the schema then lists its changelog", func(t *testing.T) {
		reg := schemamocks.NewMockSchemaRegistry(t)
		h := &handlers{schemaSvc: reg}

		current := &schema.Schema{ID: uuid.New(), Action: action, Version: 2}
		reg.EXPECT().
			Get(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
			Return(current, nil)
		reg.EXPECT().
			ListVersions(mock.Anything, current.ID, uint32(schema.MaxPageSize), uint32(0)).
			Return([]*schema.SchemaVersion{{Version: 2}, {Version: 1}}, int64(2), nil)

		res, err := h.listSchemaVersions(context.Background(), toolRequest(map[string]any{
			"ocpp_version": "1.6", "action": action, "message_type": "request",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("got error result: %v", res.Content)
		}
	})
}
