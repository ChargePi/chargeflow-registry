package grpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	grpcmocks "github.com/ChargePi/chargeflow-registry/gen/mocks/grpc"
	grpchandler "github.com/ChargePi/chargeflow-registry/internal/grpc"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListSchemaVersions(t *testing.T) {
	t.Run("rejects an invalid schema_id", func(t *testing.T) {
		svc := grpcmocks.NewMockSchemaService(t)
		h := grpchandler.NewHandler(svc)

		_, err := h.ListSchemaVersions(context.Background(), &schemav1.ListSchemaVersionsRequest{SchemaId: "not-a-uuid"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got code %v, want InvalidArgument", status.Code(err))
		}
	})

	t.Run("rejects an invalid page_token", func(t *testing.T) {
		svc := grpcmocks.NewMockSchemaService(t)
		h := grpchandler.NewHandler(svc)

		_, err := h.ListSchemaVersions(context.Background(), &schemav1.ListSchemaVersionsRequest{
			SchemaId:  uuid.New().String(),
			PageToken: "!!!not-valid-base64!!!",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got code %v, want InvalidArgument", status.Code(err))
		}
	})

	t.Run("maps a repository error to Internal", func(t *testing.T) {
		svc := grpcmocks.NewMockSchemaService(t)
		h := grpchandler.NewHandler(svc)

		id := uuid.New()
		svc.EXPECT().
			ListVersions(mock.Anything, id, uint32(schema.DefaultPageSize), uint32(0)).
			Return(nil, int64(0), errors.New("boom"))

		_, err := h.ListSchemaVersions(context.Background(), &schemav1.ListSchemaVersionsRequest{SchemaId: id.String()})
		if status.Code(err) != codes.Internal {
			t.Fatalf("got code %v, want Internal", status.Code(err))
		}
	})

	t.Run("returns the changelog on the happy path", func(t *testing.T) {
		svc := grpcmocks.NewMockSchemaService(t)
		h := grpchandler.NewHandler(svc)

		id := uuid.New()
		versions := []*schema.SchemaVersion{
			{SchemaID: id, Version: 2, Schema: json.RawMessage(`{"v":2}`), CreatedAt: time.Now()},
			{SchemaID: id, Version: 1, Schema: json.RawMessage(`{"v":1}`), CreatedAt: time.Now().Add(-time.Hour)},
		}
		svc.EXPECT().
			ListVersions(mock.Anything, id, uint32(schema.DefaultPageSize), uint32(0)).
			Return(versions, int64(2), nil)

		resp, err := h.ListSchemaVersions(context.Background(), &schemav1.ListSchemaVersionsRequest{SchemaId: id.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.TotalSize != 2 || len(resp.Versions) != 2 {
			t.Fatalf("got total=%d versions=%d, want total=2 versions=2", resp.TotalSize, len(resp.Versions))
		}
		if resp.Versions[0].Version != 2 || resp.Versions[1].Version != 1 {
			t.Fatalf("got versions in order %d, %d, want newest first: 2, 1", resp.Versions[0].Version, resp.Versions[1].Version)
		}
		if resp.NextPageToken != "" {
			t.Fatalf("got next_page_token %q, want empty since the full result fit in one page", resp.NextPageToken)
		}
	})
}
