package repositories

import (
	"context"

	"songshare/internal/models"
)

// SongRepository defines the interface for song data operations
type SongRepository interface {
	// Create and Update
	Save(ctx context.Context, song *models.Song) error
	Update(ctx context.Context, song *models.Song) error

	// Find operations
	FindByID(ctx context.Context, id string) (*models.Song, error)
	FindByISRC(ctx context.Context, isrc string) (*models.Song, error)
	FindByISRCBatch(ctx context.Context, isrcs []string) (map[string]*models.Song, error)
	FindByTitleArtist(ctx context.Context, title, artist string) ([]*models.Song, error)
	FindByPlatformID(ctx context.Context, platform, externalID string) (*models.Song, error)

	// Search operations
	Search(ctx context.Context, query string, limit int) ([]*models.Song, error)
	FindSimilar(ctx context.Context, song *models.Song, limit int) ([]*models.Song, error)
	FindByIDPrefix(ctx context.Context, prefix string) (*models.Song, error)

	// Bulk operations
	FindMany(ctx context.Context, ids []string) ([]*models.Song, error)
	SaveMany(ctx context.Context, songs []*models.Song) error

	// Maintenance operations
	DeleteByID(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}
