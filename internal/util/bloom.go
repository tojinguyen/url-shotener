package util

import (
	"context"
	"fmt"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BloomFilter is a concurrency-safe wrapper around a probabilistic set used to
// cheaply reject short keys that are guaranteed not to exist, avoiding a DB
// round-trip on the common path. Test may return false positives (never false
// negatives), so callers must confirm a positive against the database.
type BloomFilter struct {
	mu     sync.RWMutex
	filter *bloom.BloomFilter
}

// NewBloomFilter builds a filter sized for the expected capacity and target
// false-positive rate.
func NewBloomFilter(capacity uint, fpRate float64) *BloomFilter {
	return &BloomFilter{
		filter: bloom.NewWithEstimates(capacity, fpRate),
	}
}

// Add records key in the filter.
func (b *BloomFilter) Add(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter.AddString(key)
}

// Test reports whether key may be present. A false result guarantees absence.
func (b *BloomFilter) Test(key string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter.TestString(key)
}

// Warm streams every existing short_url from the collection into the filter so a
// freshly started instance retains collision protection for previously issued
// keys.
func (b *BloomFilter) Warm(ctx context.Context, coll *mongo.Collection) error {
	opts := options.Find().SetProjection(bson.M{"short_url": 1, "_id": 0})
	cursor, err := coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return fmt.Errorf("bloom warm: find: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc struct {
			ShortURL string `bson:"short_url"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("bloom warm: decode: %w", err)
		}
		if doc.ShortURL != "" {
			b.Add(doc.ShortURL)
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("bloom warm: cursor: %w", err)
	}
	return nil
}
