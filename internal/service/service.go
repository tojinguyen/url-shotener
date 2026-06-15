// Package service holds the core business logic for the write and read paths.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/toainguyen/url-shortener/internal/model"
)

// ErrNotFound indicates the short URL does not exist (or has expired).
var ErrNotFound = errors.New("short url not found")

// ErrAliasExhausted indicates a unique short key could not be generated within
// the retry budget. Mapped to HTTP 500 by handlers.
var ErrAliasExhausted = errors.New("could not generate a unique alias")

// maxRetries bounds the collision-resolution loop in the shortener.
const maxRetries = 10

// secondsPerDay is used to translate the TTL-in-days config into a duration.
const secondsPerDay = 24 * 60 * 60

// Repository is the persistence contract required by the services. It is
// satisfied by store.MongoRepository and mocked in tests. Implementations must
// return store.ErrNotFound (wrapped) from FindByShortURL when absent; callers
// detect absence via errors.Is against the package-level sentinel they expect.
type Repository interface {
	FindByShortURL(ctx context.Context, shortURL string) (*model.URL, error)
	Insert(ctx context.Context, u *model.URL) error
}

// Cache is the caching contract; satisfied by store.RedisCache. SetEx with a
// non-positive TTL is treated as a no-op. Get returns a miss error that callers
// detect via the concrete store.ErrCacheMiss sentinel.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	SetEx(ctx context.Context, key, value string, ttl time.Duration) error
}

// BloomFilter is the probabilistic membership contract; satisfied by
// util.BloomFilter.
type BloomFilter interface {
	Add(key string)
	Test(key string) bool
}
