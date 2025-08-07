package models

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Database represents the database connection
type Database struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// NewDatabase creates a new database connection
func NewDatabase(ctx context.Context, mongoURL, dbName string) (*Database, error) {
	// Set client options
	clientOptions := options.Client().
		ApplyURI(mongoURL).
		SetMaxPoolSize(20).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(30 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	db := client.Database(dbName)

	return &Database{
		Client: client,
		DB:     db,
	}, nil
}

// Close closes the database connection
func (d *Database) Close(ctx context.Context) error {
	return d.Client.Disconnect(ctx)
}

// CreateIndexes creates necessary indexes for optimal performance
func (d *Database) CreateIndexes(ctx context.Context) error {
	songsCollection := d.DB.Collection("songs")

	// Handle potential index conflicts by dropping conflicting indexes first
	err := d.handleIndexConflicts(ctx, songsCollection)
	if err != nil {
		return err
	}

	// Create indexes
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"isrc", 1}},
			Options: options.Index().SetSparse(true),
		},
		{
			Keys: bson.D{{"title", 1}, {"artist", 1}},
		},
		{
			Keys: bson.D{{"platform_links.platform", 1}, {"platform_links.external_id", 1}},
		},
		{
			Keys: bson.D{
				{"title", "text"},
				{"artist", "text"},
				{"album", "text"},
			},
			Options: options.Index().SetDefaultLanguage("english"),
		},
		{
			Keys: bson.D{{"created_at", 1}},
		},
		{
			Keys: bson.D{{"updated_at", 1}},
		},
	}

	_, err = songsCollection.Indexes().CreateMany(ctx, indexes)
	return err
}

// handleIndexConflicts checks for and resolves index conflicts
func (d *Database) handleIndexConflicts(ctx context.Context, collection *mongo.Collection) error {
	// List existing indexes
	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var existingIndexes []bson.M
	if err = cursor.All(ctx, &existingIndexes); err != nil {
		return err
	}

	// Check for conflicting ISRC index (unique vs non-unique)
	for _, index := range existingIndexes {
		if indexName, ok := index["name"].(string); ok && indexName == "isrc_1" {
			// Check if it has unique constraint
			if unique, exists := index["unique"]; exists && unique == true {
				// Drop the conflicting unique index
				_, err := collection.Indexes().DropOne(ctx, "isrc_1")
				if err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}
