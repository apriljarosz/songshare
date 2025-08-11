package cache

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/valkey-io/valkey-go"
)

// SimpleCache implements a basic Valkey cache
type SimpleCache struct {
	client valkey.Client
}

// NewSimpleCache creates a new simple Valkey-backed cache
func NewSimpleCache(valkeyURL string) (Cache, error) {
	addr, password, err := parseValkeyURL(valkeyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Valkey URL: %w", err)
	}

	clientOption := valkey.ClientOption{
		InitAddress: []string{addr},
	}

	if password != "" {
		clientOption.Password = password
	}

	client, err := valkey.NewClient(clientOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create Valkey client: %w", err)
	}

	cache := &SimpleCache{
		client: client,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cache.Health(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return cache, nil
}

// Get retrieves a value from cache
func (c *SimpleCache) Get(ctx context.Context, key string) ([]byte, error) {
	cmd := c.client.B().Get().Key(key).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		if valkey.IsValkeyNil(result.Error()) {
			return nil, nil // Key doesn't exist
		}
		return nil, &CacheError{
			Operation: "get",
			Key:       key,
			Err:       result.Error(),
		}
	}

	data, err := result.AsBytes()
	if err != nil {
		return nil, &CacheError{
			Operation: "get",
			Key:       key,
			Err:       err,
		}
	}

	return data, nil
}

// Set stores a value in cache with expiration
func (c *SimpleCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	var cmd valkey.Completed

	if expiration > 0 {
		cmd = c.client.B().Set().Key(key).Value(string(value)).Ex(expiration).Build()
	} else {
		cmd = c.client.B().Set().Key(key).Value(string(value)).Build()
	}

	result := c.client.Do(ctx, cmd)
	if result.Error() != nil {
		return &CacheError{
			Operation: "set",
			Key:       key,
			Err:       result.Error(),
		}
	}

	return nil
}

// Delete removes a key from cache
func (c *SimpleCache) Delete(ctx context.Context, key string) error {
	cmd := c.client.B().Del().Key(key).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return &CacheError{
			Operation: "delete",
			Key:       key,
			Err:       result.Error(),
		}
	}

	return nil
}

// Exists checks if a key exists in cache
func (c *SimpleCache) Exists(ctx context.Context, key string) (bool, error) {
	cmd := c.client.B().Exists().Key(key).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return false, &CacheError{
			Operation: "exists",
			Key:       key,
			Err:       result.Error(),
		}
	}

	count, err := result.AsInt64()
	if err != nil {
		return false, &CacheError{
			Operation: "exists",
			Key:       key,
			Err:       err,
		}
	}

	return count > 0, nil
}

// Close closes the Valkey connection
func (c *SimpleCache) Close() error {
	c.client.Close()
	return nil
}

// Health checks Valkey health
func (c *SimpleCache) Health(ctx context.Context) error {
	cmd := c.client.B().Ping().Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return fmt.Errorf("Valkey health check failed: %w", result.Error())
	}

	return nil
}

// parseValkeyURL extracts connection details from Valkey URL
func parseValkeyURL(valkeyURL string) (address, password string, err error) {
	u, err := url.Parse(valkeyURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Host == "" {
		return "", "", fmt.Errorf("missing host in URL")
	}
	address = u.Host

	if u.User != nil {
		password, _ = u.User.Password()
	}

	return address, password, nil
}