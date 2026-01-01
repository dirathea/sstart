// Package cache provides secret caching functionality using the system keyring.
// Secrets are cached with a configurable TTL to reduce API calls to providers.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	// KeyringService is the service name used for keyring storage
	KeyringService = "sstart-cache"
	// ConfigDirName is the name of the directory where sstart stores its configuration
	ConfigDirName = "sstart"
	// CacheFileName is the name of the cache file (fallback)
	CacheFileName = "secrets-cache.json"
	// DefaultTTL is the default cache TTL (5 minutes)
	DefaultTTL = 5 * time.Minute
)

// CachedSecrets represents cached secrets with metadata
type CachedSecrets struct {
	Secrets   map[string]string `json:"secrets"`
	ExpiresAt time.Time         `json:"expires_at"`
	CachedAt  time.Time         `json:"cached_at"`
}

// CacheStore represents the entire cache storage
type CacheStore struct {
	Providers map[string]*CachedSecrets `json:"providers"`
}

// Cache provides caching functionality for secrets
type Cache struct {
	ttl             time.Duration
	keyringDisabled bool
	keyringTested   bool
	cachePath       string
}

// Option is a functional option for configuring the Cache
type Option func(*Cache)

// WithTTL sets a custom TTL for the cache
func WithTTL(ttl time.Duration) Option {
	return func(c *Cache) {
		c.ttl = ttl
	}
}

// WithCachePath sets a custom path for file-based cache storage
func WithCachePath(path string) Option {
	return func(c *Cache) {
		c.cachePath = path
	}
}

// New creates a new Cache instance
func New(opts ...Option) *Cache {
	cache := &Cache{
		ttl:       DefaultTTL,
		cachePath: getDefaultCachePath(),
	}

	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

// getDefaultCachePath returns the default path for cache file storage
func getDefaultCachePath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", ConfigDirName, CacheFileName)
		}
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, ConfigDirName, CacheFileName)
}

// GenerateCacheKey generates a unique cache key based on provider configuration.
// The key is a hash of the provider kind, id, and configuration.
func GenerateCacheKey(providerID string, kind string, config map[string]interface{}) string {
	// Create a deterministic representation of the config
	data := map[string]interface{}{
		"provider_id": providerID,
		"kind":        kind,
		"config":      sortedConfigString(config),
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		// Fallback to simple key if marshaling fails
		return providerID
	}

	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// sortedConfigString creates a deterministic string representation of config
func sortedConfigString(config map[string]interface{}) string {
	if config == nil {
		return "{}"
	}

	// Get sorted keys
	keys := make([]string, 0, len(config))
	for k := range config {
		// Skip internal SSO tokens as they change
		if k == "_sso_access_token" || k == "_sso_id_token" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sorted representation
	result := make(map[string]interface{})
	for _, k := range keys {
		result[k] = config[k]
	}

	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Get retrieves cached secrets for a provider if they exist and are not expired
func (c *Cache) Get(cacheKey string) (map[string]string, bool) {
	store := c.loadStore()
	if store == nil {
		return nil, false
	}

	cached, exists := store.Providers[cacheKey]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		// Clean up expired entry
		delete(store.Providers, cacheKey)
		_ = c.saveStore(store)
		return nil, false
	}

	return cached.Secrets, true
}

// Set stores secrets in the cache with the configured TTL
func (c *Cache) Set(cacheKey string, secrets map[string]string) error {
	store := c.loadStore()
	if store == nil {
		store = &CacheStore{
			Providers: make(map[string]*CachedSecrets),
		}
	}

	now := time.Now()
	store.Providers[cacheKey] = &CachedSecrets{
		Secrets:   secrets,
		CachedAt:  now,
		ExpiresAt: now.Add(c.ttl),
	}

	return c.saveStore(store)
}

// Clear removes all cached secrets
func (c *Cache) Clear() error {
	var lastErr error

	// Try to clear from keyring
	if c.isKeyringAvailable() {
		if err := keyring.Delete(KeyringService, "cache"); err != nil && err != keyring.ErrNotFound {
			lastErr = fmt.Errorf("failed to remove cache from keyring: %w", err)
		}
	}

	// Also try to clear from file
	if err := os.Remove(c.cachePath); err != nil && !os.IsNotExist(err) {
		lastErr = fmt.Errorf("failed to remove cache file: %w", err)
	}

	return lastErr
}

// ClearProvider removes cached secrets for a specific provider
func (c *Cache) ClearProvider(cacheKey string) error {
	store := c.loadStore()
	if store == nil {
		return nil
	}

	delete(store.Providers, cacheKey)
	return c.saveStore(store)
}

// CleanExpired removes all expired cache entries
func (c *Cache) CleanExpired() error {
	store := c.loadStore()
	if store == nil {
		return nil
	}

	now := time.Now()
	changed := false
	for key, cached := range store.Providers {
		if now.After(cached.ExpiresAt) {
			delete(store.Providers, key)
			changed = true
		}
	}

	if changed {
		return c.saveStore(store)
	}
	return nil
}

// isKeyringAvailable checks if keyring is available on this system
func (c *Cache) isKeyringAvailable() bool {
	if c.keyringTested {
		return !c.keyringDisabled
	}

	c.keyringTested = true

	// Try to access keyring with a test operation
	_, err := keyring.Get(KeyringService, "test-availability")
	if err != nil {
		if err == keyring.ErrNotFound {
			c.keyringDisabled = false
			return true
		}
		c.keyringDisabled = true
		return false
	}

	c.keyringDisabled = false
	return true
}

// loadStore loads the cache store from keyring or file
func (c *Cache) loadStore() *CacheStore {
	// Try keyring first
	if c.isKeyringAvailable() {
		data, err := keyring.Get(KeyringService, "cache")
		if err == nil {
			var store CacheStore
			if err := json.Unmarshal([]byte(data), &store); err == nil {
				return &store
			}
			// Invalid data, clean up
			_ = keyring.Delete(KeyringService, "cache")
		}
	}

	// Fall back to file
	return c.loadStoreFromFile()
}

// loadStoreFromFile loads the cache store from a file
func (c *Cache) loadStoreFromFile() *CacheStore {
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return nil
	}

	var store CacheStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil
	}

	return &store
}

// saveStore saves the cache store to keyring or file
func (c *Cache) saveStore(store *CacheStore) error {
	data, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("failed to marshal cache store: %w", err)
	}

	// Try keyring first
	if c.isKeyringAvailable() {
		err := keyring.Set(KeyringService, "cache", string(data))
		if err == nil {
			// Clean up any old file storage
			_ = os.Remove(c.cachePath)
			return nil
		}
	}

	// Fall back to file storage
	return c.saveStoreToFile(store)
}

// saveStoreToFile saves the cache store to a file
func (c *Cache) saveStoreToFile(store *CacheStore) error {
	// Ensure directory exists
	dir := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache store: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(c.cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// GetTTL returns the configured TTL
func (c *Cache) GetTTL() time.Duration {
	return c.ttl
}

// Stats returns cache statistics
func (c *Cache) Stats() (total int, valid int, expired int) {
	store := c.loadStore()
	if store == nil {
		return 0, 0, 0
	}

	now := time.Now()
	for _, cached := range store.Providers {
		total++
		if now.Before(cached.ExpiresAt) {
			valid++
		} else {
			expired++
		}
	}
	return total, valid, expired
}
