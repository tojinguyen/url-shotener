package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/toainguyen/url-shortener/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrNotFound is returned by the repository when no document matches.
var ErrNotFound = errors.New("url not found")

// MongoRepository is the MongoDB-backed persistence layer for URL documents.
type MongoRepository struct {
	coll *mongo.Collection
}

// NewMongoRepository wraps a urls collection.
func NewMongoRepository(coll *mongo.Collection) *MongoRepository {
	return &MongoRepository{coll: coll}
}

// FindByShortURL returns the document for shortURL, or ErrNotFound.
func (r *MongoRepository) FindByShortURL(ctx context.Context, shortURL string) (*model.URL, error) {
	var doc model.URL
	err := r.coll.FindOne(ctx, bson.M{"short_url": shortURL}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find by short_url: %w", err)
	}
	return &doc, nil
}

// Insert persists a new URL document.
func (r *MongoRepository) Insert(ctx context.Context, u *model.URL) error {
	if _, err := r.coll.InsertOne(ctx, u); err != nil {
		return fmt.Errorf("insert url: %w", err)
	}
	return nil
}
