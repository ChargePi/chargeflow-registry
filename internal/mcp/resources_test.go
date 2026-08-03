package mcp

import (
	"testing"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
)

func strPtr(s string) *string { return &s }

func msgTypePtr(s string) *schema.MessageType {
	mt := schema.MessageType(s)
	return &mt
}

func TestParseSchemaURI(t *testing.T) {
	tests := []struct {
		name                                 string
		uri                                  string
		wantVersion, wantAction, wantMsgType string
		wantFilter                           schemaFilter
		wantErr                              bool
	}{
		{
			name:        "no vendor or model",
			uri:         "schema://1.6/Authorize/request",
			wantVersion: "1.6",
			wantAction:  "Authorize",
			wantMsgType: "request",
		},
		{
			name:        "vendor and model",
			uri:         "schema://1.6/Authorize/request?vendor=ABB&model=Terra54",
			wantVersion: "1.6",
			wantAction:  "Authorize",
			wantMsgType: "request",
			wantFilter:  schemaFilter{Vendor: strPtr("ABB"), Model: strPtr("Terra54")},
		},
		{
			name:        "vendor only",
			uri:         "schema://1.6/Authorize/request?vendor=ABB",
			wantVersion: "1.6",
			wantAction:  "Authorize",
			wantMsgType: "request",
			wantFilter:  schemaFilter{Vendor: strPtr("ABB")},
		},
		{
			name:    "missing type",
			uri:     "schema://1.6/Authorize",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			uri:     "schemas://1.6/Authorize/request",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, action, msgType, filter, err := parseSchemaURI(tt.uri)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if version != tt.wantVersion || action != tt.wantAction || msgType != tt.wantMsgType {
				t.Fatalf("got (%q, %q, %q), want (%q, %q, %q)", version, action, msgType, tt.wantVersion, tt.wantAction, tt.wantMsgType)
			}
			assertFilterEq(t, filter, tt.wantFilter)
		})
	}
}

func TestParseSchemasURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantVersion string
		wantFilter  schemaFilter
		wantErr     bool
	}{
		{
			name:        "version only",
			uri:         "schemas://1.6",
			wantVersion: "1.6",
		},
		{
			name:        "vendor and model",
			uri:         "schemas://1.6?vendor=ABB&model=Terra54",
			wantVersion: "1.6",
			wantFilter:  schemaFilter{Vendor: strPtr("ABB"), Model: strPtr("Terra54")},
		},
		{
			name:        "action and message_type",
			uri:         "schemas://1.6?action=Authorize&message_type=request",
			wantVersion: "1.6",
			wantFilter:  schemaFilter{Action: strPtr("Authorize"), MessageType: msgTypePtr("request")},
		},
		{
			name:        "all filters",
			uri:         "schemas://1.6?vendor=ABB&model=Terra54&action=Authorize&message_type=response",
			wantVersion: "1.6",
			wantFilter: schemaFilter{
				Vendor:      strPtr("ABB"),
				Model:       strPtr("Terra54"),
				Action:      strPtr("Authorize"),
				MessageType: msgTypePtr("response"),
			},
		},
		{
			name:    "missing version",
			uri:     "schemas://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, filter, err := parseSchemasURI(tt.uri)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if version != tt.wantVersion {
				t.Fatalf("version = %q, want %q", version, tt.wantVersion)
			}
			assertFilterEq(t, filter, tt.wantFilter)
		})
	}
}

func assertFilterEq(t *testing.T, got, want schemaFilter) {
	t.Helper()
	if !ptrEq(got.Vendor, want.Vendor) {
		t.Fatalf("vendor = %v, want %v", derefStr(got.Vendor), derefStr(want.Vendor))
	}
	if !ptrEq(got.Model, want.Model) {
		t.Fatalf("model = %v, want %v", derefStr(got.Model), derefStr(want.Model))
	}
	if !ptrEq(got.Action, want.Action) {
		t.Fatalf("action = %v, want %v", derefStr(got.Action), derefStr(want.Action))
	}
	gotMsgType, wantMsgType := "<nil>", "<nil>"
	if got.MessageType != nil {
		gotMsgType = string(*got.MessageType)
	}
	if want.MessageType != nil {
		wantMsgType = string(*want.MessageType)
	}
	if gotMsgType != wantMsgType {
		t.Fatalf("message_type = %v, want %v", gotMsgType, wantMsgType)
	}
}

func ptrEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func derefStr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
