package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewCache(client *redis.Client, ttl time.Duration) *Cache {
	return &Cache{client: client, ttl: ttl}
}

func (c *Cache) Get(ctx context.Context, key string) (*schema.Schema, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, schema.ErrNotFound
		}
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var s schema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}

	return &s, nil
}

func (c *Cache) Set(ctx context.Context, key string, s *schema.Schema) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}

	return nil
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache delete: %w", err)
	}

	return nil
}
