package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/toainguyen/url-shortener/internal/model"
	"github.com/toainguyen/url-shortener/internal/store"
)

func newRedirectorAt(repo Repository, cache Cache, now time.Time) *Redirector {
	r := NewRedirector(repo, cache)
	r.now = func() time.Time { return now }
	return r
}

func TestRedirector_Resolve_CacheHit(t *testing.T) {
	repo := &fakeRepo{}
	cache := &fakeCache{getValue: "https://example.com/cached"}

	r := NewRedirector(repo, cache)
	got, err := r.Resolve(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if got != "https://example.com/cached" {
		t.Errorf("got %q, want cached value", got)
	}
	if repo.findCalls != 0 {
		t.Errorf("expected no DB lookup on cache hit, got %d", repo.findCalls)
	}
}

func TestRedirector_Resolve_CacheMiss_DBHit_RepopulatesCache(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	expireAt := now.Add(48 * time.Hour)
	repo := &fakeRepo{findResults: []findResult{{doc: &model.URL{
		ShortURL: "abc1234",
		LongURL:  "https://example.com/db",
		ExpireAt: expireAt,
	}}}}
	cache := &fakeCache{getErr: store.ErrCacheMiss}

	r := newRedirectorAt(repo, cache, now)
	got, err := r.Resolve(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if got != "https://example.com/db" {
		t.Errorf("got %q, want DB value", got)
	}
	if cache.setExCalls != 1 {
		t.Fatalf("expected cache repopulation, got %d SetEx calls", cache.setExCalls)
	}
	if cache.lastTTL != 48*time.Hour {
		t.Errorf("repopulated TTL = %v, want remaining 48h", cache.lastTTL)
	}
}

func TestRedirector_Resolve_ExpiredDocument_NotFound(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{findResults: []findResult{{doc: &model.URL{
		ShortURL: "abc1234",
		LongURL:  "https://example.com/old",
		ExpireAt: now.Add(-1 * time.Hour), // already expired
	}}}}
	cache := &fakeCache{getErr: store.ErrCacheMiss}

	r := newRedirectorAt(repo, cache, now)
	_, err := r.Resolve(context.Background(), "abc1234")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for expired doc, got %v", err)
	}
	if cache.setExCalls != 0 {
		t.Errorf("expected no cache write for expired doc, got %d", cache.setExCalls)
	}
}

func TestRedirector_Resolve_MissingDocument_NotFound(t *testing.T) {
	repo := &fakeRepo{findResults: []findResult{{err: store.ErrNotFound}}}
	cache := &fakeCache{getErr: store.ErrCacheMiss}

	r := NewRedirector(repo, cache)
	_, err := r.Resolve(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
