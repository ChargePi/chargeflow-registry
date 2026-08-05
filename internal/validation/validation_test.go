package validation

import (
	"context"
	"testing"

	validationmocks "github.com/ChargePi/chargeflow-registry/gen/mocks/validation"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"go.uber.org/zap"
)

func TestValidateMessages_DerivesActionAndTypeFromMessage(t *testing.T) {
	getter := validationmocks.NewMockSchemaGetter(t)
	getter.EXPECT().
		Get(context.Background(), schema.OCPPVersion16, "Authorize", schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
		Return(&schema.Schema{Schema: []byte(`{"type":"object","required":["idTag"],"properties":{"idTag":{"type":"string"}}}`)}, nil)

	svc := NewService(getter, zap.NewNop())

	entries, err := svc.ValidateMessages(context.Background(), BulkRequest{
		Version: schema.OCPPVersion16,
		Messages: []string{
			`{"messageTypeId":2,"uniqueId":"1","action":"Authorize","payload":{"idTag":"ABC"}}`,
			`not json`,
			`{"messageTypeId":4,"uniqueId":"2","action":"Authorize","payload":{}}`,
			`{"messageTypeId":2,"uniqueId":"3","payload":{}}`,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}

	if !entries[0].Valid {
		t.Errorf("entry 0: want valid, got violations %+v", entries[0].Violations)
	}

	if entries[1].Valid || len(entries[1].Violations) == 0 {
		t.Errorf("entry 1 (invalid JSON): want invalid with a violation, got %+v", entries[1])
	}

	if entries[2].Valid || len(entries[2].Violations) == 0 {
		t.Errorf("entry 2 (unsupported message type): want invalid with a violation, got %+v", entries[2])
	}

	if entries[3].Valid || len(entries[3].Violations) == 0 {
		t.Errorf("entry 3 (missing action): want invalid with a violation, got %+v", entries[3])
	}
}
