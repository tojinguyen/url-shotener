package service

import (
	"context"
	"time"

	"github.com/toainguyen/url-shortener/internal/model"
	"github.com/toainguyen/url-shortener/internal/store"
)

// fakeRepo is a configurable in-memory Repository for tests.
type fakeRepo struct {
	// findResults is consumed one entry per FindByShortURL call. Each entry is
	// either a doc or an error (e.g. store.ErrNotFound).
	findResults []findResult
	findCalls   int

	inserted  []*model.URL
	insertErr error
}

type findResult struct {
	doc *model.URL
	err error
}

func (f *fakeRepo) FindByShortURL(_ context.Context, _ string) (*model.URL, error) {
	idx := f.findCalls
	f.findCalls++
	if idx < len(f.findResults) {
		r := f.findResults[idx]
		return r.doc, r.err
	}
	// Default: not found.
	return nil, store.ErrNotFound
}

func (f *fakeRepo) Insert(_ context.Context, u *model.URL) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, u)
	return nil
}

// fakeCache is a configurable Cache for tests.
type fakeCache struct {
	getValue string
	getErr   error

	setExCalls int
	lastKey    string
	lastValue  string
	lastTTL    time.Duration
	setExErr   error
}

func (c *fakeCache) Get(_ context.Context, _ string) (string, error) {
	if c.getErr != nil {
		return "", c.getErr
	}
	return c.getValue, nil
}

func (c *fakeCache) SetEx(_ context.Context, key, value string, ttl time.Duration) error {
	c.setExCalls++
	c.lastKey, c.lastValue, c.lastTTL = key, value, ttl
	return c.setExErr
}

// fakeBloom returns a fixed Test result and records Add calls.
type fakeBloom struct {
	testResult bool
	added      []string
}

func (b *fakeBloom) Add(key string)     { b.added = append(b.added, key) }
func (b *fakeBloom) Test(_ string) bool { return b.testResult }
