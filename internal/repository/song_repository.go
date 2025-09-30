package repository

import (
	"context"
	"time"

	"github.com/apriljarosz/songshare/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SongRepository struct {
	collection *mongo.Collection
}

func NewSongRepository(db *mongo.Database) *SongRepository {
	return &SongRepository{
		collection: db.Collection("songs"),
	}
}

func (r *SongRepository) FindByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	var song models.Song
	err := r.collection.FindOne(ctx, bson.M{"isrc": isrc}).Decode(&song)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &song, nil
}

func (r *SongRepository) FindByID(ctx context.Context, id string) (*models.Song, error) {
	var song models.Song
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&song)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &song, nil
}

func (r *SongRepository) Save(ctx context.Context, song *models.Song) error {
	song.UpdatedAt = time.Now()

	if song.ID == "" {
		// Generate new ID from ISRC
		song.ID = song.ISRC
		song.CreatedAt = time.Now()
	}

	upsert := true
	opts := options.Replace().SetUpsert(upsert)

	_, err := r.collection.ReplaceOne(
		ctx,
		bson.M{"_id": song.ID},
		song,
		opts,
	)
	return err
}

func (r *SongRepository) AddPlatformLink(ctx context.Context, isrc string, platform models.PlatformLink) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"isrc": isrc},
		bson.M{
			"$addToSet": bson.M{
				"platforms": platform,
			},
			"$set": bson.M{
				"updated_at": time.Now(),
			},
		},
	)
	return err
}
