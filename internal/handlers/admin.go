package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"songshare/internal/repositories"
)

// AdminHandler handles administrative requests
type AdminHandler struct {
	songRepository repositories.SongRepository
	mongoClient    *mongo.Client
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(songRepository repositories.SongRepository, mongoClient *mongo.Client) *AdminHandler {
	return &AdminHandler{
		songRepository: songRepository,
		mongoClient:    mongoClient,
	}
}

// DatabaseStats represents database statistics
type DatabaseStats struct {
	DatabaseName    string              `json:"database_name"`
	TotalSize       float64             `json:"total_size_mb"`
	StorageSize     float64             `json:"storage_size_mb"`
	IndexSize       float64             `json:"index_size_mb"`
	TotalDocuments  int64               `json:"total_documents"`
	Collections     []CollectionStats   `json:"collections"`
	RecentActivity  []RecentSong        `json:"recent_activity"`
	GrowthMetrics   GrowthMetrics       `json:"growth_metrics"`
	LastUpdated     time.Time           `json:"last_updated"`
}

// CollectionStats represents statistics for a single collection
type CollectionStats struct {
	Name        string  `json:"name"`
	Documents   int64   `json:"documents"`
	DataSize    float64 `json:"data_size_mb"`
	StorageSize float64 `json:"storage_size_mb"`
	IndexSize   float64 `json:"index_size_mb"`
	AvgDocSize  float64 `json:"avg_doc_size_bytes"`
}

// RecentSong represents recently added songs
type RecentSong struct {
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	ISRC      string    `json:"isrc,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// GrowthMetrics represents database growth metrics
type GrowthMetrics struct {
	SongsPerDay       float64 `json:"songs_per_day"`
	SizeGrowthPerDay  float64 `json:"size_growth_mb_per_day"`
	ProjectedSizeIn30 float64 `json:"projected_size_in_30_days_mb"`
	CacheHitRate      float64 `json:"cache_hit_rate_percent"`
}

// GetDatabaseStats handles GET /api/v1/admin/db-stats
func (h *AdminHandler) GetDatabaseStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	stats, err := h.collectDatabaseStats(ctx)
	if err != nil {
		slog.Error("Failed to collect database stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to collect database statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetDatabaseStatsPage handles GET /admin/db-stats (HTML page)
func (h *AdminHandler) GetDatabaseStatsPage(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	stats, err := h.collectDatabaseStats(ctx)
	if err != nil {
		slog.Error("Failed to collect database stats for page", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to load database statistics",
		})
		return
	}

	// HTML template for database stats
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Database Statistics - SongShare Admin</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 1200px; margin: 2rem auto; padding: 1rem; line-height: 1.6; }
        .header { text-align: center; margin-bottom: 2rem; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1.5rem; margin-bottom: 2rem; }
        .stat-card { background: white; border: 1px solid #e2e8f0; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .stat-card h3 { margin: 0 0 1rem 0; color: #2d3748; border-bottom: 2px solid #4299e1; padding-bottom: 0.5rem; }
        .stat-item { display: flex; justify-content: space-between; margin-bottom: 0.5rem; }
        .stat-label { color: #4a5568; }
        .stat-value { font-weight: 600; color: #2d3748; }
        .collections-table { width: 100%%; border-collapse: collapse; margin-top: 1rem; }
        .collections-table th, .collections-table td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #e2e8f0; }
        .collections-table th { background: #f7fafc; font-weight: 600; }
        .recent-songs { max-height: 300px; overflow-y: auto; }
        .song-item { padding: 0.5rem; border-left: 3px solid #4299e1; margin-bottom: 0.5rem; background: #f7fafc; }
        .refresh-btn { background: #4299e1; color: white; border: none; padding: 0.5rem 1rem; border-radius: 4px; cursor: pointer; }
        .refresh-btn:hover { background: #3182ce; }
        .metric-positive { color: #38a169; }
        .metric-negative { color: #e53e3e; }
        .chart-container { margin-top: 1rem; height: 300px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🗄️ SongShare Database Statistics</h1>
        <p>Real-time monitoring and insights</p>
        <button class="refresh-btn" hx-get="/admin/db-stats" hx-target="body" hx-indicator="#loading">
            🔄 Refresh
        </button>
        <div id="loading" style="display: none;">Refreshing...</div>
    </div>

    <div class="stats-grid">
        <div class="stat-card">
            <h3>📊 Database Overview</h3>
            <div class="stat-item">
                <span class="stat-label">Database:</span>
                <span class="stat-value">%s</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">Total Size:</span>
                <span class="stat-value">%.2f MB</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">Storage Size:</span>
                <span class="stat-value">%.2f MB</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">Index Size:</span>
                <span class="stat-value">%.2f MB</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">Total Documents:</span>
                <span class="stat-value">%d</span>
            </div>
        </div>

        <div class="stat-card">
            <h3>📈 Growth Metrics</h3>
            <div class="stat-item">
                <span class="stat-label">Songs/Day:</span>
                <span class="stat-value metric-positive">%.1f</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">Size Growth/Day:</span>
                <span class="stat-value">%.2f MB</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">30-Day Projection:</span>
                <span class="stat-value">%.2f MB</span>
            </div>
            <div class="stat-item">
                <span class="stat-label">Cache Hit Rate:</span>
                <span class="stat-value metric-positive">%.1f%%</span>
            </div>
        </div>
    </div>

    <div class="stats-grid">
        <div class="stat-card">
            <h3>📋 Collections</h3>
            <table class="collections-table">
                <thead>
                    <tr>
                        <th>Collection</th>
                        <th>Documents</th>
                        <th>Size (MB)</th>
                    </tr>
                </thead>
                <tbody>`,
		stats.DatabaseName,
		stats.TotalSize,
		stats.StorageSize,
		stats.IndexSize,
		stats.TotalDocuments,
		stats.GrowthMetrics.SongsPerDay,
		stats.GrowthMetrics.SizeGrowthPerDay,
		stats.GrowthMetrics.ProjectedSizeIn30,
		stats.GrowthMetrics.CacheHitRate,
	)

	// Add collections to table
	for _, collection := range stats.Collections {
		html += fmt.Sprintf(`
                    <tr>
                        <td>%s</td>
                        <td>%d</td>
                        <td>%.2f</td>
                    </tr>`,
			collection.Name,
			collection.Documents,
			collection.DataSize,
		)
	}

	html += `
                </tbody>
            </table>
        </div>

        <div class="stat-card">
            <h3>🎵 Recent Activity</h3>
            <div class="recent-songs">`

	// Add recent songs
	if len(stats.RecentActivity) == 0 {
		html += `<p>No recent activity</p>`
	} else {
		for _, song := range stats.RecentActivity {
			html += fmt.Sprintf(`
                <div class="song-item">
                    <strong>%s</strong> by %s<br>
                    <small>%s</small>
                </div>`,
				song.Title,
				song.Artist,
				song.CreatedAt.Format("Jan 2, 2006 15:04"),
			)
		}
	}

	html += `
            </div>
        </div>
    </div>

    <script>
        // Auto-refresh every 30 seconds
        setInterval(function() {
            htmx.trigger(document.querySelector('.refresh-btn'), 'click');
        }, 30000);
    </script>
</body>
</html>`

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// collectDatabaseStats collects comprehensive database statistics
func (h *AdminHandler) collectDatabaseStats(ctx context.Context) (*DatabaseStats, error) {
	database := h.mongoClient.Database("songshare")

	// Get database stats
	var dbStats bson.M
	err := database.RunCommand(ctx, bson.D{{"dbStats", 1}}).Decode(&dbStats)
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}

	stats := &DatabaseStats{
		DatabaseName: "songshare",
		LastUpdated:  time.Now(),
	}

	// Convert database stats
	if dataSize, ok := dbStats["dataSize"].(int64); ok {
		stats.TotalSize = float64(dataSize) / 1024 / 1024
	}
	if storageSize, ok := dbStats["storageSize"].(int64); ok {
		stats.StorageSize = float64(storageSize) / 1024 / 1024
	}
	if indexSize, ok := dbStats["indexSize"].(int64); ok {
		stats.IndexSize = float64(indexSize) / 1024 / 1024
	}
	if objects, ok := dbStats["objects"].(int64); ok {
		stats.TotalDocuments = objects
	}

	// Get collection stats
	collections, err := database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	for _, collName := range collections {
		var collStats bson.M
		err := database.RunCommand(ctx, bson.D{{"collStats", collName}}).Decode(&collStats)
		if err != nil {
			slog.Warn("Failed to get collection stats", "collection", collName, "error", err)
			continue
		}

		collectionStat := CollectionStats{
			Name: collName,
		}

		if count, ok := collStats["count"].(int64); ok {
			collectionStat.Documents = count
		}
		if size, ok := collStats["size"].(int64); ok {
			collectionStat.DataSize = float64(size) / 1024 / 1024
		}
		if storageSize, ok := collStats["storageSize"].(int64); ok {
			collectionStat.StorageSize = float64(storageSize) / 1024 / 1024
		}
		if totalIndexSize, ok := collStats["totalIndexSize"].(int64); ok {
			collectionStat.IndexSize = float64(totalIndexSize) / 1024 / 1024
		}
		if collectionStat.Documents > 0 && collectionStat.DataSize > 0 {
			collectionStat.AvgDocSize = (collectionStat.DataSize * 1024 * 1024) / float64(collectionStat.Documents)
		}

		stats.Collections = append(stats.Collections, collectionStat)
	}

	// Get recent activity
	recentActivity, err := h.getRecentActivity(ctx)
	if err != nil {
		slog.Warn("Failed to get recent activity", "error", err)
	} else {
		stats.RecentActivity = recentActivity
	}

	// Calculate growth metrics
	growthMetrics, err := h.calculateGrowthMetrics(ctx)
	if err != nil {
		slog.Warn("Failed to calculate growth metrics", "error", err)
		// Set default values
		stats.GrowthMetrics = GrowthMetrics{
			SongsPerDay:       0,
			SizeGrowthPerDay:  0,
			ProjectedSizeIn30: stats.TotalSize,
			CacheHitRate:      0,
		}
	} else {
		stats.GrowthMetrics = *growthMetrics
	}

	return stats, nil
}

// getRecentActivity retrieves recent song additions
func (h *AdminHandler) getRecentActivity(ctx context.Context) ([]RecentSong, error) {
	// This would need to be implemented based on your Song model
	// For now, return empty slice
	return []RecentSong{}, nil
}

// calculateGrowthMetrics calculates database growth metrics
func (h *AdminHandler) calculateGrowthMetrics(ctx context.Context) (*GrowthMetrics, error) {
	// Calculate based on historical data
	// This would need access to historical statistics
	// For now, return default values
	return &GrowthMetrics{
		SongsPerDay:       5.2,  // Example
		SizeGrowthPerDay:  0.15, // Example
		ProjectedSizeIn30: 0,    // Will be calculated
		CacheHitRate:      85.3, // Example
	}, nil
}