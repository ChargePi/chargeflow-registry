package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("schema.cache")

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewCache(client *redis.Client, ttl time.Duration) *Cache {
	return &Cache{client: client, ttl: ttl}
}

func (c *Cache) Get(ctx context.Context, key string) (*schema.Schema, error) {
	ctx, span := tracer.Start(ctx, "cache.Get", trace.WithAttributes(attribute.String("cache.key", key)))
	defer span.End()

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			span.SetAttributes(attribute.Bool("cache.hit", false))
			return nil, schema.ErrNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var s schema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	return &s, nil
}

func (c *Cache) Set(ctx context.Context, key string, s *schema.Schema) error {
	ctx, span := tracer.Start(ctx, "cache.Set", trace.WithAttributes(attribute.String("cache.key", key)))
	defer span.End()

	data, err := json.Marshal(s)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cache set: %w", err)
	}

	return nil
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "cache.Delete", trace.WithAttributes(attribute.String("cache.key", key)))
	defer span.End()

	if err := c.client.Del(ctx, key).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("cache delete: %w", err)
	}

	return nil
}
