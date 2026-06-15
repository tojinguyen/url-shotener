// Package store wires the MongoDB and Redis clients used by the services.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/toainguyen/url-shortener/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMongo connects to MongoDB and verifies the connection with a ping.
func NewMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	return client, nil
}

// URLCollection returns the handle to the urls collection.
func URLCollection(client *mongo.Client, dbName string) *mongo.Collection {
	return client.Database(dbName).Collection(model.CollectionName)
}

// EnsureIndexes creates the unique index on short_url and the TTL index on
// expire_at. It is idempotent and safe to call on every startup.
func EnsureIndexes(ctx context.Context, coll *mongo.Collection) error {
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "short_url", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_short_url"),
		},
		{
			// MongoDB removes the document once expire_at is in the past.
			Keys:    bson.D{{Key: "expire_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_expire_at"),
		},
	}

	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

// DisconnectTimeout bounds graceful client shutdown.
const DisconnectTimeout = 5 * time.Second
