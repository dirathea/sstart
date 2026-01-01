package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		kind       string
		config     map[string]interface{}
	}{
		{
			name:       "simple config",
			providerID: "aws-prod",
			kind:       "aws_secretsmanager",
			config: map[string]interface{}{
				"region": "us-east-1",
				"secret": "my-secret",
			},
		},
		{
			name:       "empty config",
			providerID: "dotenv",
			kind:       "dotenv",
			config:     map[string]interface{}{},
		},
		{
			name:       "config with SSO tokens should be ignored",
			providerID: "vault",
			kind:       "vault",
			config: map[string]interface{}{
				"address":           "https://vault.example.com",
				"_sso_access_token": "token123",
				"_sso_id_token":     "idtoken456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GenerateCacheKey(tt.providerID, tt.kind, tt.config)
			if key == "" {
				t.Error("expected non-empty cache key")
			}
			// Key should be deterministic
			key2 := GenerateCacheKey(tt.providerID, tt.kind, tt.config)
			if key != key2 {
				t.Errorf("cache key should be deterministic, got %s and %s", key, key2)
			}
		})
	}
}

func TestGenerateCacheKey_DifferentConfigs(t *testing.T) {
	config1 := map[string]interface{}{"region": "us-east-1"}
	config2 := map[string]interface{}{"region": "us-west-2"}

	key1 := GenerateCacheKey("aws", "aws_secretsmanager", config1)
	key2 := GenerateCacheKey("aws", "aws_secretsmanager", config2)

	if key1 == key2 {
		t.Error("different configs should produce different cache keys")
	}
}

func TestGenerateCacheKey_SSOTokensIgnored(t *testing.T) {
	configWithoutToken := map[string]interface{}{
		"address": "https://vault.example.com",
	}
	configWithToken := map[string]interface{}{
		"address":           "https://vault.example.com",
		"_sso_access_token": "token123",
		"_sso_id_token":     "idtoken456",
	}

	key1 := GenerateCacheKey("vault", "vault", configWithoutToken)
	key2 := GenerateCacheKey("vault", "vault", configWithToken)

	if key1 != key2 {
		t.Error("SSO tokens should be ignored when generating cache key")
	}
}

func TestCache_SetAndGet(t *testing.T) {
	// Create a temporary directory for file-based cache
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := New(WithCachePath(cachePath), WithTTL(time.Minute))

	secrets := map[string]string{
		"API_KEY":     "secret123",
		"DB_PASSWORD": "dbpass456",
	}

	cacheKey := "test-key-123"

	// Set secrets
	err := cache.Set(cacheKey, secrets)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Get secrets
	cached, found := cache.Get(cacheKey)
	if !found {
		t.Fatal("expected to find cached secrets")
	}

	if len(cached) != len(secrets) {
		t.Errorf("expected %d secrets, got %d", len(secrets), len(cached))
	}

	for k, v := range secrets {
		if cached[k] != v {
			t.Errorf("expected %s=%s, got %s=%s", k, v, k, cached[k])
		}
	}
}

func TestCache_Expiration(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Use a very short TTL
	c := New(WithCachePath(cachePath), WithTTL(50*time.Millisecond))
	// Force file-based cache for isolation
	c.keyringTested = true
	c.keyringDisabled = true

	secrets := map[string]string{"KEY": "value"}
	cacheKey := "expiring-key"

	err := c.Set(cacheKey, secrets)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Should be found immediately
	_, found := c.Get(cacheKey)
	if !found {
		t.Fatal("expected to find cached secrets immediately after setting")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, found = c.Get(cacheKey)
	if found {
		t.Error("expected cache to be expired")
	}
}

func TestCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := New(WithCachePath(cachePath))

	secrets := map[string]string{"KEY": "value"}
	cacheKey := "clear-test-key"

	_ = cache.Set(cacheKey, secrets)

	// Verify it's set
	_, found := cache.Get(cacheKey)
	if !found {
		t.Fatal("expected to find cached secrets before clear")
	}

	// Clear cache
	err := cache.Clear()
	if err != nil {
		t.Fatalf("failed to clear cache: %v", err)
	}

	// Should not be found after clear
	_, found = cache.Get(cacheKey)
	if found {
		t.Error("expected cache to be cleared")
	}
}

func TestCache_ClearProvider(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := New(WithCachePath(cachePath))

	secrets1 := map[string]string{"KEY1": "value1"}
	secrets2 := map[string]string{"KEY2": "value2"}

	_ = cache.Set("provider1", secrets1)
	_ = cache.Set("provider2", secrets2)

	// Clear only provider1
	err := cache.ClearProvider("provider1")
	if err != nil {
		t.Fatalf("failed to clear provider: %v", err)
	}

	// provider1 should be gone
	_, found := cache.Get("provider1")
	if found {
		t.Error("expected provider1 to be cleared")
	}

	// provider2 should still exist
	_, found = cache.Get("provider2")
	if !found {
		t.Error("expected provider2 to still exist")
	}
}

func TestCache_CleanExpired(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := New(WithCachePath(cachePath), WithTTL(50*time.Millisecond))

	secrets := map[string]string{"KEY": "value"}
	_ = cache.Set("expiring", secrets)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Add a fresh entry
	cache2 := New(WithCachePath(cachePath), WithTTL(time.Hour))
	_ = cache2.Set("fresh", map[string]string{"KEY2": "value2"})

	// Clean expired
	err := cache2.CleanExpired()
	if err != nil {
		t.Fatalf("failed to clean expired: %v", err)
	}

	// Expired should be gone
	_, found := cache2.Get("expiring")
	if found {
		t.Error("expected expired entry to be cleaned")
	}

	// Fresh should still exist
	_, found = cache2.Get("fresh")
	if !found {
		t.Error("expected fresh entry to still exist")
	}
}

func TestCache_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	c := New(WithCachePath(cachePath), WithTTL(50*time.Millisecond))
	// Force file-based cache for isolation
	c.keyringTested = true
	c.keyringDisabled = true

	// Initially empty
	total, valid, expired := c.Stats()
	if total != 0 || valid != 0 || expired != 0 {
		t.Errorf("expected empty stats, got total=%d, valid=%d, expired=%d", total, valid, expired)
	}

	// Add entries
	_ = c.Set("key1", map[string]string{"K": "V"})
	_ = c.Set("key2", map[string]string{"K": "V"})

	total, valid, expired = c.Stats()
	if total != 2 || valid != 2 || expired != 0 {
		t.Errorf("expected 2 valid entries, got total=%d, valid=%d, expired=%d", total, valid, expired)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	total, valid, expired = c.Stats()
	if total != 2 || valid != 0 || expired != 2 {
		t.Errorf("expected 2 expired entries, got total=%d, valid=%d, expired=%d", total, valid, expired)
	}
}

func TestCache_GetTTL(t *testing.T) {
	cache := New(WithTTL(10 * time.Minute))
	if cache.GetTTL() != 10*time.Minute {
		t.Errorf("expected TTL of 10m, got %v", cache.GetTTL())
	}

	cache2 := New() // Default TTL
	if cache2.GetTTL() != DefaultTTL {
		t.Errorf("expected default TTL of %v, got %v", DefaultTTL, cache2.GetTTL())
	}
}

func TestCache_FileFallback(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := New(WithCachePath(cachePath))
	// Force disable keyring for this test
	cache.keyringTested = true
	cache.keyringDisabled = true

	secrets := map[string]string{"KEY": "value"}
	err := cache.Set("file-test", secrets)
	if err != nil {
		t.Fatalf("failed to set cache with file fallback: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("expected cache file to be created")
	}

	// Should be able to read back
	cached, found := cache.Get("file-test")
	if !found {
		t.Fatal("expected to find cached secrets from file")
	}

	if cached["KEY"] != "value" {
		t.Errorf("expected KEY=value, got KEY=%s", cached["KEY"])
	}
}

func TestCache_CorruptedCacheFile(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create a cache file with empty JSON (no providers key)
	if err := os.WriteFile(cachePath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write corrupted cache file: %v", err)
	}

	c := New(WithCachePath(cachePath))
	c.keyringTested = true
	c.keyringDisabled = true

	// This should NOT panic even with corrupted cache file
	secrets := map[string]string{"KEY": "value"}
	err := c.Set("test-key", secrets)
	if err != nil {
		t.Fatalf("failed to set cache with corrupted file: %v", err)
	}

	// Should be able to read back
	cached, found := c.Get("test-key")
	if !found {
		t.Fatal("expected to find cached secrets")
	}

	if cached["KEY"] != "value" {
		t.Errorf("expected KEY=value, got KEY=%s", cached["KEY"])
	}
}

func TestCache_NullProvidersInFile(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create a cache file with null providers
	if err := os.WriteFile(cachePath, []byte(`{"providers": null}`), 0600); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	c := New(WithCachePath(cachePath))
	c.keyringTested = true
	c.keyringDisabled = true

	// This should NOT panic
	secrets := map[string]string{"KEY": "value"}
	err := c.Set("test-key", secrets)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	cached, found := c.Get("test-key")
	if !found {
		t.Fatal("expected to find cached secrets")
	}

	if cached["KEY"] != "value" {
		t.Errorf("expected KEY=value, got KEY=%s", cached["KEY"])
	}
}
