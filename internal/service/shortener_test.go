package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/toainguyen/url-shortener/internal/model"
	"github.com/toainguyen/url-shortener/internal/store"
)

const ttlDays = 30

func TestShortener_Create_BloomMiss_FastPath(t *testing.T) {
	repo := &fakeRepo{}
	cache := &fakeCache{}
	bloom := &fakeBloom{testResult: false} // guaranteed-not-found -> no DB probe

	s := NewShortener(repo, cache, bloom, ttlDays)
	doc, err := s.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if repo.findCalls != 0 {
		t.Errorf("expected no DB probe on bloom miss, got %d FindByShortURL calls", repo.findCalls)
	}
	if len(doc.ShortURL) != model.ShortKeyLength {
		t.Errorf("short key length = %d, want %d", len(doc.ShortURL), model.ShortKeyLength)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected 1 inserted doc, got %d", len(repo.inserted))
	}
	if cache.setExCalls != 1 {
		t.Errorf("expected cache SetEx called once, got %d", cache.setExCalls)
	}
	wantTTL := time.Duration(ttlDays) * 24 * time.Hour
	if cache.lastTTL != wantTTL {
		t.Errorf("cache TTL = %v, want %v", cache.lastTTL, wantTTL)
	}
	if len(bloom.added) != 1 || bloom.added[0] != doc.ShortURL {
		t.Errorf("expected bloom updated with %q, got %v", doc.ShortURL, bloom.added)
	}
}

func TestShortener_Create_BloomFalsePositive_DBConfirmsFree(t *testing.T) {
	repo := &fakeRepo{findResults: []findResult{{err: store.ErrNotFound}}}
	cache := &fakeCache{}
	bloom := &fakeBloom{testResult: true} // forces DB confirmation

	s := NewShortener(repo, cache, bloom, ttlDays)
	doc, err := s.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.findCalls != 1 {
		t.Errorf("expected exactly 1 DB probe, got %d", repo.findCalls)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].ShortURL != doc.ShortURL {
		t.Errorf("expected doc inserted with key %q", doc.ShortURL)
	}
}

func TestShortener_Create_CollisionRetry_ThenSucceeds(t *testing.T) {
	// First probe finds an existing doc (collision); second probe is free.
	repo := &fakeRepo{findResults: []findResult{
		{doc: &model.URL{ShortURL: "AAAAAAA"}}, // collision on attempt 0
		{err: store.ErrNotFound},               // free on attempt 1 (salted)
	}}
	cache := &fakeCache{}
	bloom := &fakeBloom{testResult: true}

	s := NewShortener(repo, cache, bloom, ttlDays)
	doc, err := s.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.findCalls != 2 {
		t.Errorf("expected 2 DB probes (retry), got %d", repo.findCalls)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected 1 inserted doc after retry, got %d", len(repo.inserted))
	}
	if doc.ShortURL == "" {
		t.Error("expected a non-empty short key")
	}
}

func TestShortener_Create_ExhaustsRetries_Returns500Error(t *testing.T) {
	// Every probe reports an existing collision -> loop exhausts after maxRetries.
	results := make([]findResult, maxRetries)
	for i := range results {
		results[i] = findResult{doc: &model.URL{ShortURL: "COLLIDE"}}
	}
	repo := &fakeRepo{findResults: results}
	cache := &fakeCache{}
	bloom := &fakeBloom{testResult: true}

	s := NewShortener(repo, cache, bloom, ttlDays)
	_, err := s.Create(context.Background(), "https://example.com")
	if !errors.Is(err, ErrAliasExhausted) {
		t.Fatalf("expected ErrAliasExhausted, got %v", err)
	}
	if repo.findCalls != maxRetries {
		t.Errorf("expected %d DB probes, got %d", maxRetries, repo.findCalls)
	}
	if len(repo.inserted) != 0 {
		t.Errorf("expected no insert on exhaustion, got %d", len(repo.inserted))
	}
}

func TestShortener_Create_AppliesTTLExpiry(t *testing.T) {
	repo := &fakeRepo{}
	cache := &fakeCache{}
	bloom := &fakeBloom{testResult: false}

	fixed := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	s := NewShortener(repo, cache, bloom, ttlDays)
	s.now = func() time.Time { return fixed }

	doc, err := s.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	want := fixed.AddDate(0, 0, ttlDays)
	if !doc.ExpireAt.Equal(want) {
		t.Errorf("ExpireAt = %v, want %v", doc.ExpireAt, want)
	}
}
