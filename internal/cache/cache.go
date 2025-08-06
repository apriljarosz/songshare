package cache

import (
	"context"
	"time"
)

// Cache defines the interface for caching operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) ([]byte, error)
	
	// Set stores a value in cache with expiration
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	
	// Delete removes a key from cache
	Delete(ctx context.Context, key string) error
	
	// Exists checks if a key exists in cache
	Exists(ctx context.Context, key string) (bool, error)
	
	// Close closes the cache connection
	Close() error
	
	// Health checks cache health
	Health(ctx context.Context) error
}

// CacheError represents a cache operation error
type CacheError struct {
	Operation string
	Key       string
	Err       error
}

func (e *CacheError) Error() string {
	return "cache " + e.Operation + " failed for key '" + e.Key + "': " + e.Err.Error()
}

func (e *CacheError) Unwrap() error {
	return e.Err
}