package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/toainguyen/url-shortener/internal/model"
	"github.com/toainguyen/url-shortener/internal/store"
	"github.com/toainguyen/url-shortener/internal/util"
)

// Shortener implements the write path: generate a unique short key and persist
// the mapping across MongoDB, Redis, and the in-memory Bloom filter.
type Shortener struct {
	repo    Repository
	cache   Cache
	bloom   BloomFilter
	ttlDays int
	now     func() time.Time // injectable clock for deterministic tests
}

// NewShortener constructs a Shortener.
func NewShortener(repo Repository, cache Cache, bloom BloomFilter, ttlDays int) *Shortener {
	return &Shortener{
		repo:    repo,
		cache:   cache,
		bloom:   bloom,
		ttlDays: ttlDays,
		now:     time.Now,
	}
}

// Create generates a unique 7-character short key for longURL, persists the
// mapping, seeds the cache, and updates the Bloom filter. It follows the
// specified collision-resolution sequence: hash the URL, probe the Bloom filter,
// and only consult MongoDB on a potential collision.
func (s *Shortener) Create(ctx context.Context, longURL string) (*model.URL, error) {
	expireAt := s.now().AddDate(0, 0, s.ttlDays)

	shortKey, err := s.generateUniqueKey(ctx, longURL)
	if err != nil {
		return nil, err
	}

	doc := &model.URL{
		ShortURL:  shortKey,
		LongURL:   longURL,
		CreatedAt: s.now(),
		ExpireAt:  expireAt,
	}
	if err := s.repo.Insert(ctx, doc); err != nil {
		return nil, fmt.Errorf("shortener: persist: %w", err)
	}

	// Seed the cache with the full configured TTL. A cache failure must not fail
	// the write — the redirector can lazily repopulate from MongoDB.
	ttl := time.Duration(s.ttlDays) * secondsPerDay * time.Second
	if err := s.cache.SetEx(ctx, shortKey, longURL, ttl); err != nil {
		// Non-fatal: log-and-continue semantics are left to the caller's logger.
		_ = err
	}

	s.bloom.Add(shortKey)
	return doc, nil
}

// generateUniqueKey runs the bounded retry loop described in the spec.
func (s *Shortener) generateUniqueKey(ctx context.Context, longURL string) (string, error) {
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		seed := longURL
		if retryCount > 0 {
			// Salt with a high-resolution timestamp to break the deterministic
			// collision on subsequent attempts.
			seed = longURL + strconv.FormatInt(s.now().UnixNano(), 10)
		}
		shortKey := util.Base62Key(util.MD5(seed), model.ShortKeyLength)

		// Bloom filter says "definitely not present" -> the key is free.
		if !s.bloom.Test(shortKey) {
			return shortKey, nil
		}

		// Potential collision: confirm against the database.
		_, err := s.repo.FindByShortURL(ctx, shortKey)
		if errors.Is(err, store.ErrNotFound) {
			// False positive — the key is actually free.
			return shortKey, nil
		}
		if err != nil {
			return "", fmt.Errorf("shortener: collision check: %w", err)
		}
		// A record exists; retry with a fresh salt.
	}

	return "", ErrAliasExhausted
}
