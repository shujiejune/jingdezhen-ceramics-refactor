package tokenblocklist

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// keyPrefix is the Redis namespace for revoked users.
const keyPrefix = "jwt:revoked:"

// RedisBlocklist is the production Blocklist backed by Redis. It is safe for
// concurrent use (go-redis is goroutine-safe; one client per process).
type RedisBlocklist struct {
	client *redis.Client
}

// NewRedisBlocklist wraps a go-redis client. The caller owns the client and must
// Close() it on shutdown (the blocklist shares the app-wide Redis client).
func NewRedisBlocklist(client *redis.Client) *RedisBlocklist {
	return &RedisBlocklist{client: client}
}

// Revoke SETs jwt:revoked:<userID> with a TTL of ttl. Idempotent.
func (b *RedisBlocklist) Revoke(ctx context.Context, userID string, ttl time.Duration) error {
	if b.client == nil {
		return nil // defensive: nil client behaves as Noop
	}
	if err := b.client.Set(ctx, keyPrefix+userID, "1", ttl).Err(); err != nil {
		return fmt.Errorf("tokenblocklist.Revoke: %w", err)
	}
	return nil
}

// IsRevoked reports whether the user's outstanding tokens were revoked. On a
// Redis error it logs and returns (false, nil) — fail-open so a Redis outage
// never locks out every authenticated request (a deleted user's token still
// self-expires within MaxAccessTokenTTL and login is already blocked).
func (b *RedisBlocklist) IsRevoked(ctx context.Context, userID string) (bool, error) {
	if b.client == nil {
		return false, nil
	}
	n, err := b.client.Exists(ctx, keyPrefix+userID).Result()
	if err != nil {
		log.Printf("tokenblocklist.IsRevoked: redis error (fail-open): %v", err)
		return false, nil
	}
	return n > 0, nil
}
