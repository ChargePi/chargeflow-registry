package schema_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	schemamocks "github.com/ChargePi/chargeflow-registry/gen/mocks/schema"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

func TestServiceListVersions(t *testing.T) {
	t.Run("clamps zero limit to default page size", func(t *testing.T) {
		repo := schemamocks.NewMockRepository(t)
		cache := schemamocks.NewMockCache(t)
		svc := schema.NewService(repo, cache)

		id := uuid.New()
		repo.EXPECT().
			ListVersions(mock.Anything, id, uint32(schema.DefaultPageSize), uint32(0)).
			Return([]*schema.SchemaVersion{{Version: 1}}, int64(1), nil)

		versions, total, err := svc.ListVersions(context.Background(), id, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(versions) != 1 {
			t.Fatalf("got total=%d versions=%d, want total=1 versions=1", total, len(versions))
		}
	})

	t.Run("wraps repository errors", func(t *testing.T) {
		repo := schemamocks.NewMockRepository(t)
		cache := schemamocks.NewMockCache(t)
		svc := schema.NewService(repo, cache)

		id := uuid.New()
		repoErr := errors.New("boom")
		repo.EXPECT().
			ListVersions(mock.Anything, id, uint32(schema.DefaultPageSize), uint32(0)).
			Return(nil, int64(0), repoErr)

		_, _, err := svc.ListVersions(context.Background(), id, 0, 0)
		if !errors.Is(err, repoErr) {
			t.Fatalf("got error %v, want it to wrap %v", err, repoErr)
		}
	})
}

func TestServiceGetVersion(t *testing.T) {
	action := "BootNotification"
	current := &schema.Schema{
		ID:          uuid.New(),
		OCPPVersion: schema.OCPPVersion16,
		Action:      action,
		MessageType: schema.MessageTypeRequest,
		Status:      schema.StatusVerified,
		Schema:      json.RawMessage(`{"v":2}`),
		Version:     2,
		CreatedAt:   time.Now().Add(-time.Hour),
		UpdatedAt:   time.Now(),
	}

	t.Run("short-circuits when the current version already matches", func(t *testing.T) {
		repo := schemamocks.NewMockRepository(t)
		cache := schemamocks.NewMockCache(t)
		svc := schema.NewService(repo, cache)

		cache.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, schema.ErrNotFound)
		cache.EXPECT().Set(mock.Anything, mock.Anything, current).Return(nil)
		repo.EXPECT().
			Get(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
			Return(current, nil)
		// GetVersion must NOT be called: no expectation set on repo for it.

		got, err := svc.GetVersion(context.Background(), schema.OCPPVersion16, action, schema.MessageTypeRequest, nil, nil, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != current {
			t.Fatalf("got %v, want the current schema returned as-is", got)
		}
	})

	t.Run("fetches archived content for an older version", func(t *testing.T) {
		repo := schemamocks.NewMockRepository(t)
		cache := schemamocks.NewMockCache(t)
		svc := schema.NewService(repo, cache)

		archivedAt := current.CreatedAt.Add(time.Minute)
		archived := &schema.SchemaVersion{
			SchemaID:  current.ID,
			Version:   1,
			Schema:    json.RawMessage(`{"v":1}`),
			CreatedAt: archivedAt,
		}

		cache.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, schema.ErrNotFound)
		cache.EXPECT().Set(mock.Anything, mock.Anything, current).Return(nil)
		repo.EXPECT().
			Get(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
			Return(current, nil)
		repo.EXPECT().GetVersion(mock.Anything, current.ID, 1).Return(archived, nil)

		got, err := svc.GetVersion(context.Background(), schema.OCPPVersion16, action, schema.MessageTypeRequest, nil, nil, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != current.ID || got.Status != current.Status {
			t.Fatalf("got identity fields %+v, want them copied from the current row", got)
		}
		if got.Version != 1 || string(got.Schema) != string(archived.Schema) {
			t.Fatalf("got version=%d schema=%s, want the archived content", got.Version, got.Schema)
		}
		if !got.UpdatedAt.Equal(archivedAt) {
			t.Fatalf("got UpdatedAt=%v, want it set to the archived entry's CreatedAt=%v", got.UpdatedAt, archivedAt)
		}
	})

	t.Run("propagates not-found for a version that doesn't exist", func(t *testing.T) {
		repo := schemamocks.NewMockRepository(t)
		cache := schemamocks.NewMockCache(t)
		svc := schema.NewService(repo, cache)

		cache.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, schema.ErrNotFound)
		cache.EXPECT().Set(mock.Anything, mock.Anything, current).Return(nil)
		repo.EXPECT().
			Get(mock.Anything, schema.OCPPVersion16, action, schema.MessageTypeRequest, (*string)(nil), (*string)(nil)).
			Return(current, nil)
		repo.EXPECT().GetVersion(mock.Anything, current.ID, 99).Return(nil, schema.ErrNotFound)

		_, err := svc.GetVersion(context.Background(), schema.OCPPVersion16, action, schema.MessageTypeRequest, nil, nil, 99)
		if !errors.Is(err, schema.ErrNotFound) {
			t.Fatalf("got error %v, want it to wrap ErrNotFound", err)
		}
	})
}
