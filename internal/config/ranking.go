package config

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// RankingConfig holds tunable weights for search ranking/scoring
type RankingConfig struct {
	// Scales how much raw popularity (0-100) contributes in the engine ranker
	// Example: 0.8 means popularity contributes up to 80 points
	RankerPopularityScale float64 `toml:"ranker_popularity_scale"`

	// Platform preference weights used as tertiary tiebreakers
	PlatformWeights map[string]float64 `toml:"platform_weights"`

	// Consider scores within this epsilon as ties, then break using popularity
	TieEpsilon float64 `toml:"tie_epsilon"`

	// Multiplier applied to the scorer's popularity boost after thresholding
	// 1.0 keeps default behavior; >1.0 increases popularity influence
	PopularityBoostMultiplier float64 `toml:"popularity_boost_multiplier"`

	// Weights for aggregating popularity across platforms for the same ISRC
	// Used by scorer when computing a single popularity from multiple platforms
	PopularityPlatformWeights map[string]float64 `toml:"popularity_platform_weights"`
}

// DefaultRankingConfig returns hard-coded safe defaults
func DefaultRankingConfig() *RankingConfig {
	return &RankingConfig{
		RankerPopularityScale: 0.8,
		PlatformWeights: map[string]float64{
			"local":       1.2,
			"spotify":     1.1,
			"apple_music": 1.0,
			"tidal":       0.9,
		},
		TieEpsilon:                2.5,
		PopularityBoostMultiplier: 1.0,
		PopularityPlatformWeights: map[string]float64{
			"spotify":     1.0,
			"tidal":       0.8,
			"apple_music": 0.0,
		},
	}
}

var (
	rankingCfg     *RankingConfig
	rankingCfgOnce sync.Once
	rankingCfgMu   sync.RWMutex
)

// GetRankingConfig loads the ranking config from TOML if RANKING_CONFIG_PATH is set.
// Falls back to defaults if the env var is unset or the file cannot be read/parsed.
func GetRankingConfig() *RankingConfig {
	rankingCfgOnce.Do(func() {
		cfg := DefaultRankingConfig()
		// Priority 1: explicit env var
		if path := os.Getenv("RANKING_CONFIG_PATH"); path != "" {
			if fileCfg, err := loadRankingConfigFromPath(path); err == nil && fileCfg != nil {
				mergeRankingConfig(cfg, fileCfg)
			}
		} else {
			// Priority 2: well-known default locations
			for _, p := range candidateRankingConfigPaths() {
				if fileCfg, err := loadRankingConfigFromPath(p); err == nil && fileCfg != nil {
					mergeRankingConfig(cfg, fileCfg)
					break
				}
			}
		}
		rankingCfgMu.Lock()
		rankingCfg = cfg
		rankingCfgMu.Unlock()
	})
	rankingCfgMu.RLock()
	cfg := rankingCfg
	rankingCfgMu.RUnlock()
	return cfg
}

func loadRankingConfigFromPath(path string) (*RankingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg RankingConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func mergeRankingConfig(base, override *RankingConfig) {
	if override == nil || base == nil {
		return
	}
	if override.RankerPopularityScale > 0 {
		base.RankerPopularityScale = override.RankerPopularityScale
	}
	if override.PlatformWeights != nil {
		if base.PlatformWeights == nil {
			base.PlatformWeights = map[string]float64{}
		}
		for k, v := range override.PlatformWeights {
			base.PlatformWeights[k] = v
		}
	}
	if override.TieEpsilon > 0 {
		base.TieEpsilon = override.TieEpsilon
	}
	if override.PopularityBoostMultiplier > 0 {
		base.PopularityBoostMultiplier = override.PopularityBoostMultiplier
	}
	if override.PopularityPlatformWeights != nil {
		if base.PopularityPlatformWeights == nil {
			base.PopularityPlatformWeights = map[string]float64{}
		}
		for k, v := range override.PopularityPlatformWeights {
			base.PopularityPlatformWeights[k] = v
		}
	}
}

// candidateRankingConfigPaths returns common locations to auto-discover ranking config
func candidateRankingConfigPaths() []string {
	var paths []string
	// Current working directory
	paths = append(paths,
		"ranking.toml",
		filepath.Join("config", "ranking.toml"),
	)

	// XDG config home
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "songshare", "ranking.toml"))
	}

	// User config under HOME
	if home := os.Getenv("HOME"); home != "" {
		paths = append(paths, filepath.Join(home, ".config", "songshare", "ranking.toml"))
	}

	// System-wide fallback
	paths = append(paths, filepath.Join(string(os.PathSeparator), "etc", "songshare", "ranking.toml"))
	return paths
}

// StartRankingConfigWatcher polls the ranking config file for changes and reloads it.
// If a path is provided via RANKING_CONFIG_PATH, that is used. Otherwise, the first
// existing path from candidateRankingConfigPaths is used. If no file exists, the
// watcher is a no-op.
func StartRankingConfigWatcher(ctx context.Context, interval time.Duration) {
	// Determine watched path
	paths := []string{}
	if explicit := os.Getenv("RANKING_CONFIG_PATH"); explicit != "" {
		paths = append(paths, explicit)
	} else {
		paths = append(paths, candidateRankingConfigPaths()...)
	}

	var watchPath string
	var lastModTime time.Time
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			watchPath = p
			lastModTime = fi.ModTime()
			break
		}
	}
	if watchPath == "" {
		slog.Info("ranking config watcher: no config file found; using defaults")
		return
	}

	slog.Info("ranking config watcher: watching file", "path", watchPath)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("ranking config watcher: stopped")
				return
			case <-ticker.C:
				fi, err := os.Stat(watchPath)
				if err != nil || fi.IsDir() {
					continue
				}
				if fi.ModTime().After(lastModTime) {
					if fileCfg, err := loadRankingConfigFromPath(watchPath); err == nil && fileCfg != nil {
						// Merge over defaults to keep unspecified keys sane
						newCfg := DefaultRankingConfig()
						mergeRankingConfig(newCfg, fileCfg)
						rankingCfgMu.Lock()
						rankingCfg = newCfg
						rankingCfgMu.Unlock()
						lastModTime = fi.ModTime()
						slog.Info("ranking config reloaded", "path", watchPath, "mtime", lastModTime)
					}
				}
			}
		}
	}()
}
