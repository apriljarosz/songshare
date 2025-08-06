package repositories

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"songshare/internal/models"
)

// mongoSongRepository implements SongRepository interface using MongoDB
type mongoSongRepository struct {
	collection *mongo.Collection
}

// NewMongoSongRepository creates a new MongoDB-backed song repository
func NewMongoSongRepository(db *models.Database) SongRepository {
	return &mongoSongRepository{
		collection: db.DB.Collection("songs"),
	}
}

// Save creates a new song or updates existing one
func (r *mongoSongRepository) Save(ctx context.Context, song *models.Song) error {
	song.SchemaVersion = models.CurrentSchemaVersion
	song.UpdatedAt = time.Now()

	if song.ID.IsZero() {
		// New song
		song.CreatedAt = time.Now()
		result, err := r.collection.InsertOne(ctx, song)
		if err != nil {
			return fmt.Errorf("failed to insert song: %w", err)
		}
		song.ID = result.InsertedID.(primitive.ObjectID)
		return nil
	}

	// Update existing song
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": song.ID}, song)
	if err != nil {
		return fmt.Errorf("failed to update song: %w", err)
	}
	return nil
}

// Update updates an existing song
func (r *mongoSongRepository) Update(ctx context.Context, song *models.Song) error {
	if song.ID.IsZero() {
		return fmt.Errorf("song ID is required for update")
	}

	song.UpdatedAt = time.Now()
	song.SchemaVersion = models.CurrentSchemaVersion

	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": song.ID}, song)
	if err != nil {
		return fmt.Errorf("failed to update song: %w", err)
	}
	return nil
}

// FindByID finds a song by its ObjectID
func (r *mongoSongRepository) FindByID(ctx context.Context, id string) (*models.Song, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid object ID: %w", err)
	}

	var song models.Song
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&song)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find song by ID: %w", err)
	}

	r.handleSchemaEvolution(&song)
	return &song, nil
}

// FindByISRC finds a song by its ISRC code
func (r *mongoSongRepository) FindByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	var song models.Song
	err := r.collection.FindOne(ctx, bson.M{"isrc": isrc}).Decode(&song)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find song by ISRC: %w", err)
	}

	r.handleSchemaEvolution(&song)
	return &song, nil
}

// FindByTitleArtist finds songs by title and artist (fuzzy matching)
func (r *mongoSongRepository) FindByTitleArtist(ctx context.Context, title, artist string) ([]*models.Song, error) {
	// Create case-insensitive regex patterns
	titlePattern := primitive.Regex{Pattern: regexp.QuoteMeta(title), Options: "i"}
	artistPattern := primitive.Regex{Pattern: regexp.QuoteMeta(artist), Options: "i"}

	filter := bson.M{
		"title":  titlePattern,
		"artist": artistPattern,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find songs by title/artist: %w", err)
	}
	defer cursor.Close(ctx)

	var songs []*models.Song
	for cursor.Next(ctx) {
		var song models.Song
		if err := cursor.Decode(&song); err != nil {
			slog.Error("Failed to decode song", "error", err)
			continue
		}
		r.handleSchemaEvolution(&song)
		songs = append(songs, &song)
	}

	return songs, cursor.Err()
}

// FindByPlatformID finds a song by platform-specific ID
func (r *mongoSongRepository) FindByPlatformID(ctx context.Context, platform, externalID string) (*models.Song, error) {
	filter := bson.M{
		"platform_links": bson.M{
			"$elemMatch": bson.M{
				"platform":    platform,
				"external_id": externalID,
			},
		},
	}

	var song models.Song
	err := r.collection.FindOne(ctx, filter).Decode(&song)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find song by platform ID: %w", err)
	}

	r.handleSchemaEvolution(&song)
	return &song, nil
}

// Search performs full-text search on songs
func (r *mongoSongRepository) Search(ctx context.Context, query string, limit int) ([]*models.Song, error) {
	filter := bson.M{
		"$text": bson.M{
			"$search": query,
		},
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"score": bson.M{"$meta": "textScore"}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search songs: %w", err)
	}
	defer cursor.Close(ctx)

	var songs []*models.Song
	for cursor.Next(ctx) {
		var song models.Song
		if err := cursor.Decode(&song); err != nil {
			slog.Error("Failed to decode song", "error", err)
			continue
		}
		r.handleSchemaEvolution(&song)
		songs = append(songs, &song)
	}

	return songs, cursor.Err()
}

// FindSimilar finds similar songs using aggregation pipeline
func (r *mongoSongRepository) FindSimilar(ctx context.Context, song *models.Song, limit int) ([]*models.Song, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"_id": bson.M{"$ne": song.ID}, // Exclude the input song
				"$or": []bson.M{
					{"isrc": song.ISRC},
					{
						"$and": []bson.M{
							{"title": primitive.Regex{Pattern: regexp.QuoteMeta(song.Title), Options: "i"}},
							{"artist": primitive.Regex{Pattern: regexp.QuoteMeta(song.Artist), Options: "i"}},
						},
					},
				},
			},
		},
		{
			"$addFields": bson.M{
				"similarity_score": bson.M{
					"$add": []interface{}{
						// ISRC match = 100 points
						bson.M{"$cond": []interface{}{
							bson.M{"$eq": []interface{}{"$isrc", song.ISRC}}, 100, 0}},
						// Exact title match = 50 points
						bson.M{"$cond": []interface{}{
							bson.M{"$eq": []interface{}{"$title", song.Title}}, 50, 0}},
						// Exact artist match = 30 points
						bson.M{"$cond": []interface{}{
							bson.M{"$eq": []interface{}{"$artist", song.Artist}}, 30, 0}},
					},
				},
			},
		},
		{
			"$sort": bson.M{"similarity_score": -1},
		},
		{
			"$limit": int64(limit),
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar songs: %w", err)
	}
	defer cursor.Close(ctx)

	var songs []*models.Song
	for cursor.Next(ctx) {
		var song models.Song
		if err := cursor.Decode(&song); err != nil {
			slog.Error("Failed to decode song", "error", err)
			continue
		}
		r.handleSchemaEvolution(&song)
		songs = append(songs, &song)
	}

	return songs, cursor.Err()
}

// FindMany finds multiple songs by their IDs
func (r *mongoSongRepository) FindMany(ctx context.Context, ids []string) ([]*models.Song, error) {
	objectIDs := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue // Skip invalid IDs
		}
		objectIDs = append(objectIDs, objectID)
	}

	if len(objectIDs) == 0 {
		return []*models.Song{}, nil
	}

	filter := bson.M{"_id": bson.M{"$in": objectIDs}}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find songs: %w", err)
	}
	defer cursor.Close(ctx)

	var songs []*models.Song
	for cursor.Next(ctx) {
		var song models.Song
		if err := cursor.Decode(&song); err != nil {
			slog.Error("Failed to decode song", "error", err)
			continue
		}
		r.handleSchemaEvolution(&song)
		songs = append(songs, &song)
	}

	return songs, cursor.Err()
}

// SaveMany saves multiple songs in bulk
func (r *mongoSongRepository) SaveMany(ctx context.Context, songs []*models.Song) error {
	if len(songs) == 0 {
		return nil
	}

	now := time.Now()
	docs := make([]interface{}, len(songs))
	
	for i, song := range songs {
		song.SchemaVersion = models.CurrentSchemaVersion
		song.UpdatedAt = now
		if song.CreatedAt.IsZero() {
			song.CreatedAt = now
		}
		docs[i] = song
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("failed to save songs in bulk: %w", err)
	}

	return nil
}

// DeleteByID deletes a song by its ID
func (r *mongoSongRepository) DeleteByID(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid object ID: %w", err)
	}

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return fmt.Errorf("failed to delete song: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("song not found")
	}

	return nil
}

// Count returns the total number of songs in the collection
func (r *mongoSongRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count songs: %w", err)
	}
	return count, nil
}

// handleSchemaEvolution handles schema migration for older documents
func (r *mongoSongRepository) handleSchemaEvolution(song *models.Song) {
	if song.SchemaVersion >= models.CurrentSchemaVersion {
		return
	}

	// Handle migration from older schema versions
	switch song.SchemaVersion {
	case 0:
		// Migration from version 0 to 1
		// Add any necessary field transformations here
		song.SchemaVersion = 1
		fallthrough
	default:
		song.SchemaVersion = models.CurrentSchemaVersion
	}

	// Lazy update the document in the database
	// This could be done in a background process if preferred
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.Update(ctx, song); err != nil {
			slog.Error("Failed to update song schema version", "songID", song.ID, "error", err)
		}
	}()
}