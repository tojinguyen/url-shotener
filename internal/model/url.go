// Package model defines the persistence and transport types for the service.
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ShortKeyLength is the fixed length of every generated short key.
const ShortKeyLength = 7

// CollectionName is the MongoDB collection that stores URL mappings.
const CollectionName = "urls"

// URL is the BSON document persisted in MongoDB.
type URL struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	ShortURL  string             `bson:"short_url" json:"short_url"`
	LongURL   string             `bson:"long_url" json:"long_url"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	ExpireAt  time.Time          `bson:"expire_at" json:"expire_at"`
}

// ShortenRequest is the JSON payload for POST /api/v1/shorten.
type ShortenRequest struct {
	LongURL string `json:"long_url"`
}

// ShortenResponse is returned (201) after a short URL is created.
type ShortenResponse struct {
	ShortURL string    `json:"short_url"`
	LongURL  string    `json:"long_url"`
	ExpireAt time.Time `json:"expire_at"`
}
