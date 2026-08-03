package mcp

import "testing"

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
