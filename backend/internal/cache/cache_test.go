package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCache implements the Cache interface for testing
type MockCache struct {
	data map[string][]byte
}

func NewMockCache() Cache {
	return &MockCache{
		data: make(map[string][]byte),
	}
}

func (m *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, &CacheError{Operation: "get", Key: key, Err: assert.AnError}
}

func (m *MockCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockCache) Close() error {
	m.data = nil
	return nil
}

func (m *MockCache) Health(ctx context.Context) error {
	return nil
}

func TestCacheInterface_Basic(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Test Set and Get
	err := cache.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	// Test Exists
	exists, err := cache.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Test missing key
	_, err = cache.Get(ctx, "missing")
	assert.Error(t, err)

	var cacheErr *CacheError
	assert.ErrorAs(t, err, &cacheErr)
	assert.Equal(t, "get", cacheErr.Operation)
	assert.Equal(t, "missing", cacheErr.Key)
}

func TestCacheInterface_Delete(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Set a value
	err := cache.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	// Verify it exists
	exists, err := cache.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete the key
	err = cache.Delete(ctx, "key1")
	require.NoError(t, err)

	// Verify it's gone
	exists, err = cache.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, exists)

	// Getting deleted key should return error
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
}

func TestCacheInterface_Overwrite(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Set initial value
	err := cache.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	// Overwrite with new value
	err = cache.Set(ctx, "key1", []byte("value2"), time.Hour)
	require.NoError(t, err)

	value, err = cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value2"), value)
}

func TestCacheInterface_Health(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	err := cache.Health(ctx)
	assert.NoError(t, err)
}

func TestCacheInterface_Close(t *testing.T) {
	cache := NewMockCache()

	err := cache.Close()
	assert.NoError(t, err)
}

func TestCacheError_Error(t *testing.T) {
	err := &CacheError{
		Operation: "get",
		Key:       "test-key",
		Err:       assert.AnError,
	}

	expectedMessage := "cache get failed for key 'test-key': assert.AnError general error for testing"
	assert.Equal(t, expectedMessage, err.Error())
}

func TestCacheError_Unwrap(t *testing.T) {
	wrappedErr := assert.AnError
	err := &CacheError{
		Operation: "set",
		Key:       "test-key",
		Err:       wrappedErr,
	}

	assert.Equal(t, wrappedErr, err.Unwrap())
}

func TestCacheInterface_WithContext(t *testing.T) {
	cache := NewMockCache()
	defer cache.Close()

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	// These operations should complete before timeout
	err := cache.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)
}

func TestCacheInterface_EmptyValues(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Test empty byte slice
	err := cache.Set(ctx, "empty", []byte{}, time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "empty")
	require.NoError(t, err)
	assert.Equal(t, []byte{}, value)

	// Test empty key
	err = cache.Set(ctx, "", []byte("value"), time.Hour)
	require.NoError(t, err)

	value, err = cache.Get(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), value)
}

func TestCacheInterface_BinaryData(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Test with binary data
	binaryData := []byte{0x00, 0x01, 0xFF, 0xFE, 0x80}
	err := cache.Set(ctx, "binary", binaryData, time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "binary")
	require.NoError(t, err)
	assert.Equal(t, binaryData, value)
}

func TestCacheInterface_LongKeys(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Test with very long key
	longKey := string(make([]byte, 1000))
	for i := range longKey {
		longKey = longKey[:i] + "a" + longKey[i+1:]
	}

	err := cache.Set(ctx, longKey, []byte("value"), time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, longKey)
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), value)
}

func TestCacheInterface_ExpirationParameter(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Test different expiration values (mock doesn't enforce expiration, but should accept them)
	testCases := []time.Duration{
		0,                // No expiration
		time.Minute,      // 1 minute
		time.Hour,        // 1 hour
		24 * time.Hour,   // 1 day
		-1 * time.Minute, // Negative duration
	}

	for i, expiration := range testCases {
		key := "key" + string(rune(i))
		err := cache.Set(ctx, key, []byte("value"), expiration)
		assert.NoError(t, err, "Failed to set with expiration %v", expiration)

		// Verify value was set
		value, err := cache.Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, []byte("value"), value)
	}
}

// Benchmark tests for cache interface
func BenchmarkCache_Set(b *testing.B) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	data := []byte("benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + string(rune(i%1000))
		cache.Set(ctx, key, data, time.Hour)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	data := []byte("benchmark test data")

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := "key" + string(rune(i))
		cache.Set(ctx, key, data, time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + string(rune(i%1000))
		cache.Get(ctx, key)
	}
}

func BenchmarkCache_Exists(b *testing.B) {
	ctx := context.Background()
	cache := NewMockCache()
	defer cache.Close()

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := "key" + string(rune(i))
		cache.Set(ctx, key, []byte("data"), time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + string(rune(i%1000))
		cache.Exists(ctx, key)
	}
}
