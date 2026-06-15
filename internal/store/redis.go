package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrCacheMiss is returned by Cache.Get when the key is absent.
var ErrCacheMiss = errors.New("cache miss")

// Cache abstracts the Redis operations the services depend on, enabling mocking
// in unit tests.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	SetEx(ctx context.Context, key, value string, ttl time.Duration) error
}

// RedisCache is the production Cache backed by go-redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedis connects to Redis and verifies the connection with a ping.
func NewRedis(ctx context.Context, addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisCache{client: client}, nil
}

// Get returns the value for key, or ErrCacheMiss if it does not exist.
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("redis get: %w", err)
	}
	return val, nil
}

// SetEx stores key=value with the given TTL. A non-positive TTL is ignored to
// avoid writing a key that Redis would treat as "no expiry".
func (c *RedisCache) SetEx(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis setex: %w", err)
	}
	return nil
}

// Close releases the underlying connection pool.
func (c *RedisCache) Close() error {
	return c.client.Close()
}
