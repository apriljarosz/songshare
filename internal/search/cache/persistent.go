package cache

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PersistentCache handles MongoDB-based search result caching
type PersistentCache struct {
	collection *mongo.Collection
	config     CacheConfig
}

// NewPersistentCache creates a new persistent cache using MongoDB
func NewPersistentCache(database *mongo.Database, config CacheConfig) *PersistentCache {
	collection := database.Collection("search_cache")
	
	// Create indexes for efficient querying
	pc := &PersistentCache{
		collection: collection,
		config:     config,
	}
	
	// Initialize indexes
	if err := pc.ensureIndexes(context.Background()); err != nil {
		// Log error but don't fail initialization
		fmt.Printf("Warning: Failed to create search cache indexes: %v\n", err)
	}
	
	return pc
}

// ensureIndexes creates necessary indexes for the search cache collection
func (pc *PersistentCache) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "query_hash", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(0), // TTL index
		},
		{
			Keys: bson.D{
				{Key: "hit_count", Value: -1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "platforms", Value: 1},
				{Key: "result_count", Value: -1},
			},
		},
	}
	
	_, err := pc.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// GetSearchResults retrieves cached search results by query hash
func (pc *PersistentCache) GetSearchResults(ctx context.Context, queryHash string) ([]SearchResult, bool) {
	var cached CachedSearchResult
	
	filter := bson.M{
		"query_hash": queryHash,
		"expires_at": bson.M{"$gt": time.Now()}, // Not expired
	}
	
	err := pc.collection.FindOne(ctx, filter).Decode(&cached)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false
		}
		return nil, false
	}
	
	// Update hit count and last accessed time
	go pc.incrementHitCount(context.Background(), queryHash)
	
	return cached.Results, true
}

// StoreSearchResults stores search results in the persistent cache
func (pc *PersistentCache) StoreSearchResults(ctx context.Context, queryHash string, req SearchRequest, results []SearchResult) error {
	now := time.Now()
	expiresAt := now.Add(pc.config.MongoTTL)
	
	// Determine platforms that were searched
	platforms := make(map[string]bool)
	for _, result := range results {
		if result.Platform != "" {
			platforms[result.Platform] = true
		}
	}
	
	platformList := make([]string, 0, len(platforms))
	for platform := range platforms {
		platformList = append(platformList, platform)
	}
	
	cached := CachedSearchResult{
		QueryHash:   queryHash,
		Query:       req,
		Results:     results,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   expiresAt,
		HitCount:    1,
		Platforms:   platformList,
		ResultCount: len(results),
	}
	
	// Use upsert to handle duplicate queries
	filter := bson.M{"query_hash": queryHash}
	update := bson.M{
		"$set": cached,
		"$setOnInsert": bson.M{
			"created_at": now,
			"hit_count":  1,
		},
	}
	
	opts := options.Update().SetUpsert(true)
	_, err := pc.collection.UpdateOne(ctx, filter, update, opts)
	
	return err
}

// InvalidateQuery removes cached results for a specific query
func (pc *PersistentCache) InvalidateQuery(ctx context.Context, queryHash string) error {
	filter := bson.M{"query_hash": queryHash}
	_, err := pc.collection.DeleteOne(ctx, filter)
	return err
}

// GetPopularQueries returns the most frequently searched queries
func (pc *PersistentCache) GetPopularQueries(ctx context.Context, limit int) ([]SearchRequest, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"expires_at": bson.M{"$gt": time.Now()},
				"hit_count":  bson.M{"$gt": 1}, // Only queries hit more than once
			},
		},
		{
			"$sort": bson.M{
				"hit_count":  -1,
				"updated_at": -1,
			},
		},
		{
			"$limit": limit,
		},
		{
			"$project": bson.M{
				"query": 1,
			},
		},
	}
	
	cursor, err := pc.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var queries []SearchRequest
	for cursor.Next(ctx) {
		var doc struct {
			Query SearchRequest `bson:"query"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		queries = append(queries, doc.Query)
	}
	
	return queries, nil
}

// CleanupExpired removes expired cache entries (backup for TTL index)
func (pc *PersistentCache) CleanupExpired(ctx context.Context) (int64, error) {
	filter := bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
	}
	
	result, err := pc.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	
	return result.DeletedCount, nil
}

// GetStats returns cache usage statistics
func (pc *PersistentCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id": nil,
				"total_entries": bson.M{"$sum": 1},
				"total_hits":    bson.M{"$sum": "$hit_count"},
				"avg_results":   bson.M{"$avg": "$result_count"},
				"expired": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$lt": bson.A{"$expires_at", time.Now()}},
							1,
							0,
						},
					},
				},
			},
		},
	}
	
	cursor, err := pc.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var stats map[string]interface{}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&stats); err != nil {
			return nil, err
		}
	}
	
	// Add platform distribution
	platformStats, _ := pc.getPlatformStats(ctx)
	if platformStats != nil {
		stats["platform_distribution"] = platformStats
	}
	
	return stats, nil
}

// getPlatformStats returns distribution of cached results by platform
func (pc *PersistentCache) getPlatformStats(ctx context.Context) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$unwind": "$platforms",
		},
		{
			"$group": bson.M{
				"_id":   "$platforms",
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
	}
	
	cursor, err := pc.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	platformStats := make(map[string]interface{})
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		platformStats[doc.ID] = doc.Count
	}
	
	return platformStats, nil
}

// incrementHitCount updates the hit count for a cached query
func (pc *PersistentCache) incrementHitCount(ctx context.Context, queryHash string) error {
	filter := bson.M{"query_hash": queryHash}
	update := bson.M{
		"$inc": bson.M{"hit_count": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}
	
	_, err := pc.collection.UpdateOne(ctx, filter, update)
	return err
}

// Close closes the persistent cache connection
func (pc *PersistentCache) Close() error {
	// MongoDB connections are typically managed at the database level
	// This is a no-op but implements the interface
	return nil
}