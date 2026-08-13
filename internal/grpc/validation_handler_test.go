package grpc

import (
	"context"
	"errors"
	"testing"

	grpcmocks "github.com/ChargePi/chargeflow-registry/gen/mocks/grpc"
	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/ChargePi/chargeflow-registry/internal/validation"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBulkValidateMessages_ValidationErrors(t *testing.T) {
	model := "Terra54"

	tests := []struct {
		name string
		req  *schemav1.BulkValidateMessagesRequest
		code codes.Code
	}{
		{
			name: "missing ocpp_version",
			req: &schemav1.BulkValidateMessagesRequest{
				Messages: []string{"{}"},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "model without vendor",
			req: &schemav1.BulkValidateMessagesRequest{
				OcppVersion: schemav1.OcppVersion_OCPP_VERSION_16,
				Model:       &model,
				Messages:    []string{"{}"},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "no messages",
			req: &schemav1.BulkValidateMessagesRequest{
				OcppVersion: schemav1.OcppVersion_OCPP_VERSION_16,
				Messages:    nil,
			},
			code: codes.InvalidArgument,
		},
		{
			name: "too many messages",
			req: &schemav1.BulkValidateMessagesRequest{
				OcppVersion: schemav1.OcppVersion_OCPP_VERSION_16,
				Messages:    make([]string, validation.MaxBulkMessages+1),
			},
			code: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewValidationHandler(grpcmocks.NewMockValidationService(t))

			_, err := h.BulkValidateMessages(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if got := status.Code(err); got != tt.code {
				t.Errorf("status code = %v, want %v", got, tt.code)
			}
		})
	}
}

func TestBulkValidateMessages_OrderedReport(t *testing.T) {
	svc := grpcmocks.NewMockValidationService(t)
	h := NewValidationHandler(svc)

	req := &schemav1.BulkValidateMessagesRequest{
		OcppVersion: schemav1.OcppVersion_OCPP_VERSION_16,
		Messages: []string{
			`{"messageTypeId":2,"uniqueId":"1","action":"Authorize","payload":{}}`,
			`{"messageTypeId":2,"uniqueId":"2","action":"Authorize","payload":{}}`,
			`{"messageTypeId":3,"uniqueId":"1","action":"Authorize","payload":{}}`,
		},
	}

	want := []validation.ReportEntry{
		{Index: 0, Valid: true},
		{Index: 1, Valid: false, Violations: []validation.Violation{{Field: "/idTag", Message: "required"}}},
		{Index: 2, Valid: true},
	}

	svc.EXPECT().
		ValidateMessages(context.Background(), validation.BulkRequest{
			Version:  schema.OCPPVersion16,
			Messages: req.Messages,
		}).
		Return(want, nil)

	resp, err := h.BulkValidateMessages(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != len(want) {
		t.Fatalf("got %d results, want %d", len(resp.Results), len(want))
	}
	for i, entry := range resp.Results {
		if entry.Index != int32(want[i].Index) {
			t.Errorf("result[%d].Index = %d, want %d", i, entry.Index, want[i].Index)
		}
		if entry.Valid != want[i].Valid {
			t.Errorf("result[%d].Valid = %v, want %v", i, entry.Valid, want[i].Valid)
		}
		if len(entry.Violations) != len(want[i].Violations) {
			t.Errorf("result[%d].Violations count = %d, want %d", i, len(entry.Violations), len(want[i].Violations))
		}
	}
}

func TestBulkValidateMessages_ServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "not found", err: schema.ErrNotFound, code: codes.NotFound},
		{name: "internal", err: errors.New("boom"), code: codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := grpcmocks.NewMockValidationService(t)
			h := NewValidationHandler(svc)

			svc.EXPECT().
				ValidateMessages(mock.Anything, mock.Anything).
				Return(nil, tt.err)

			_, err := h.BulkValidateMessages(context.Background(), &schemav1.BulkValidateMessagesRequest{
				OcppVersion: schemav1.OcppVersion_OCPP_VERSION_16,
				Messages:    []string{"{}"},
			})
			if got := status.Code(err); got != tt.code {
				t.Errorf("status code = %v, want %v", got, tt.code)
			}
		})
	}
}
