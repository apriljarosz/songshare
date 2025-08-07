package cache

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"
)

// valkeyCache implements Cache interface using Valkey
type valkeyCache struct {
	client valkey.Client
}

// NewValkeyCache creates a new Valkey-backed cache
func NewValkeyCache(valkeyURL string) (Cache, error) {
	// Parse Valkey URL and create client options
	addr, password, err := parseValkeyURL(valkeyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Valkey URL: %w", err)
	}

	clientOption := valkey.ClientOption{
		InitAddress: []string{addr},
	}
	
	// Add authentication if password is provided
	if password != "" {
		clientOption.Password = password
	}

	client, err := valkey.NewClient(clientOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create Valkey client: %w", err)
	}

	cache := &valkeyCache{
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

// Get retrieves a value from Valkey
func (c *valkeyCache) Get(ctx context.Context, key string) ([]byte, error) {
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

// Set stores a value in Valkey with expiration
func (c *valkeyCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
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

// Delete removes a key from Valkey
func (c *valkeyCache) Delete(ctx context.Context, key string) error {
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

// Exists checks if a key exists in Valkey
func (c *valkeyCache) Exists(ctx context.Context, key string) (bool, error) {
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
func (c *valkeyCache) Close() error {
	c.client.Close()
	return nil
}

// Health checks Valkey health
func (c *valkeyCache) Health(ctx context.Context) error {
	cmd := c.client.B().Ping().Build()
	result := c.client.Do(ctx, cmd)
	
	if result.Error() != nil {
		return fmt.Errorf("Valkey health check failed: %w", result.Error())
	}

	return nil
}

// parseValkeyURL extracts connection details from Valkey URL
func parseValkeyURL(valkeyURL string) (address, password string, err error) {
	// Parse URL properly
	u, err := url.Parse(valkeyURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL format: %w", err)
	}
	
	// Extract host:port
	if u.Host == "" {
		return "", "", fmt.Errorf("missing host in URL")
	}
	address = u.Host
	
	// Extract password if present
	if u.User != nil {
		password, _ = u.User.Password()
	}
	
	return address, password, nil
}

// MultiLevelCache implements a multi-level cache with in-memory L1 and Valkey L2
type MultiLevelCache struct {
	l1Cache    map[string]cacheItem
	l2Cache    Cache
	l1MaxItems int
	mu         sync.RWMutex // Protects l1Cache
}

type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(valkeyURL string, l1MaxItems int) (Cache, error) {
	l2Cache, err := NewValkeyCache(valkeyURL)
	if err != nil {
		return nil, err
	}

	return &MultiLevelCache{
		l1Cache:    make(map[string]cacheItem),
		l2Cache:    l2Cache,
		l1MaxItems: l1MaxItems,
	}, nil
}

// Get retrieves from L1 first, then L2
func (c *MultiLevelCache) Get(ctx context.Context, key string) ([]byte, error) {
	// Check L1 cache first
	c.mu.RLock()
	if item, exists := c.l1Cache[key]; exists {
		if time.Now().Before(item.expiresAt) {
			c.mu.RUnlock()
			return item.data, nil
		}
		// Expired, need to remove - upgrade to write lock
		c.mu.RUnlock()
		c.mu.Lock()
		// Double-check after acquiring write lock
		if item, exists := c.l1Cache[key]; exists && !time.Now().Before(item.expiresAt) {
			delete(c.l1Cache, key)
		}
		c.mu.Unlock()
	} else {
		c.mu.RUnlock()
	}

	// Check L2 cache
	data, err := c.l2Cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data != nil {
		// Populate L1 cache
		c.setL1(key, data, time.Hour) // Default L1 expiration
	}

	return data, nil
}

// Set stores in both L1 and L2
func (c *MultiLevelCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	// Set in L2 first
	if err := c.l2Cache.Set(ctx, key, value, expiration); err != nil {
		return err
	}

	// Set in L1 with shorter expiration
	l1Expiration := expiration
	if l1Expiration > time.Hour {
		l1Expiration = time.Hour
	}
	c.setL1(key, value, l1Expiration)

	return nil
}

// Delete removes from both levels
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	// Remove from L1
	c.mu.Lock()
	delete(c.l1Cache, key)
	c.mu.Unlock()

	// Remove from L2
	return c.l2Cache.Delete(ctx, key)
}

// Exists checks both levels
func (c *MultiLevelCache) Exists(ctx context.Context, key string) (bool, error) {
	// Check L1 first
	c.mu.RLock()
	if item, exists := c.l1Cache[key]; exists && time.Now().Before(item.expiresAt) {
		c.mu.RUnlock()
		return true, nil
	}
	c.mu.RUnlock()

	// Check L2
	return c.l2Cache.Exists(ctx, key)
}

// Close closes L2 connection
func (c *MultiLevelCache) Close() error {
	return c.l2Cache.Close()
}

// Health checks L2 health
func (c *MultiLevelCache) Health(ctx context.Context) error {
	return c.l2Cache.Health(ctx)
}

// setL1 sets a value in L1 cache with basic LRU eviction
func (c *MultiLevelCache) setL1(key string, value []byte, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple eviction: if at max capacity, remove oldest
	if len(c.l1Cache) >= c.l1MaxItems {
		// Find oldest item to evict (simplified approach)
		oldestKey := ""
		oldestTime := time.Now()
		
		for k, item := range c.l1Cache {
			if item.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = item.expiresAt
			}
		}
		
		if oldestKey != "" {
			delete(c.l1Cache, oldestKey)
		}
	}

	c.l1Cache[key] = cacheItem{
		data:      value,
		expiresAt: time.Now().Add(expiration),
	}
}