// Package redis provides the shared Redis client used for sessions, cache,
// and Asynq job queues. One client per process; pooled by go-redis.
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// NewClient builds a go-redis client from a `redis://...` URL (REDIS_URL).
// The caller owns the returned client and must Close() it on shutdown.
func NewClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("platform/redis: parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("platform/redis: ping: %w", err)
	}
	return client, nil
}
