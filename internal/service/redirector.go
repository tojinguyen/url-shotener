package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/toainguyen/url-shortener/internal/store"
)

// Redirector implements the read path with a cache-aside strategy backed by an
// explicit expiry guard.
type Redirector struct {
	repo  Repository
	cache Cache
	now   func() time.Time // injectable clock for deterministic tests
}

// NewRedirector constructs a Redirector.
func NewRedirector(repo Repository, cache Cache) *Redirector {
	return &Redirector{repo: repo, cache: cache, now: time.Now}
}

// Resolve returns the long URL for shortURL. It checks Redis first; on a miss it
// queries MongoDB, validates expiry (guarding against TTL-cleanup lag), lazily
// repopulates the cache with the remaining TTL, and returns the destination.
// Returns ErrNotFound when the mapping is absent or expired.
func (r *Redirector) Resolve(ctx context.Context, shortURL string) (string, error) {
	// Fast path: cache hit.
	longURL, err := r.cache.Get(ctx, shortURL)
	if err == nil {
		return longURL, nil
	}
	if !errors.Is(err, store.ErrCacheMiss) {
		return "", fmt.Errorf("redirector: cache get: %w", err)
	}

	// Cache miss: consult MongoDB.
	doc, err := r.repo.FindByShortURL(ctx, shortURL)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("redirector: lookup: %w", err)
	}

	// Explicit expiry check: MongoDB's TTL sweep is asynchronous and may lag.
	remaining := doc.ExpireAt.Sub(r.now())
	if remaining <= 0 {
		return "", ErrNotFound
	}

	// Lazily repopulate the cache for the remaining lifetime. Non-fatal on error.
	if err := r.cache.SetEx(ctx, shortURL, doc.LongURL, remaining); err != nil {
		_ = err
	}

	return doc.LongURL, nil
}
