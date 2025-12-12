package doppler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/dirathea/sstart/internal/provider"
)

// DopplerConfig represents the configuration for Doppler provider
type DopplerConfig struct {
	// Project is the Doppler project name (required)
	Project string `json:"project" yaml:"project"`
	// Config is the Doppler config/environment name (required, e.g., "dev", "staging", "prod")
	Config string `json:"config" yaml:"config"`
	// APIHost is the Doppler API host (optional, defaults to "https://api.doppler.com")
	APIHost string `json:"api_host,omitempty" yaml:"api_host,omitempty"`
}

// DopplerProvider implements the provider interface for Doppler
type DopplerProvider struct {
	client *http.Client
}

func init() {
	provider.Register("doppler", func() provider.Provider {
		return &DopplerProvider{
			client: &http.Client{
				Timeout: 30 * time.Second,
			},
		}
	})
}

// Name returns the provider name
func (p *DopplerProvider) Name() string {
	return "doppler"
}

// Fetch fetches secrets from Doppler
func (p *DopplerProvider) Fetch(ctx context.Context, mapID string, config map[string]interface{}, keys map[string]string) ([]provider.KeyValue, error) {
	// Convert map to strongly typed config struct
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid doppler configuration: %w", err)
	}

	// Validate required fields
	if cfg.Project == "" {
		return nil, fmt.Errorf("doppler provider requires 'project' field in configuration")
	}
	if cfg.Config == "" {
		return nil, fmt.Errorf("doppler provider requires 'config' field in configuration")
	}

	// Get service token from environment
	serviceToken := os.Getenv("DOPPLER_TOKEN")
	if serviceToken == "" {
		return nil, fmt.Errorf("doppler provider requires 'DOPPLER_TOKEN' environment variable")
	}

	// Set default API host if not provided
	apiHost := cfg.APIHost
	if apiHost == "" {
		apiHost = "https://api.doppler.com"
	}

	// Build API URL
	apiURL := fmt.Sprintf("%s/v3/configs/config/secrets/download?format=json&project=%s&config=%s",
		apiHost, cfg.Project, cfg.Config)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", serviceToken))
	req.Header.Set("Accept", "application/json")

	// Make HTTP request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secrets from Doppler: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("doppler API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var secretData map[string]interface{}
	if err := json.Unmarshal(body, &secretData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Map keys according to configuration
	kvs := make([]provider.KeyValue, 0)
	for k, v := range secretData {
		targetKey := k

		// Check if there's a specific mapping
		if mappedKey, exists := keys[k]; exists {
			if mappedKey == "==" {
				targetKey = k // Keep same name
			} else {
				targetKey = mappedKey
			}
		} else if len(keys) == 0 {
			// No keys specified means map everything
			targetKey = k
		} else {
			// Skip keys not in the mapping
			continue
		}

		// Convert value to string
		var value string
		switch val := v.(type) {
		case string:
			value = val
		case []byte:
			value = string(val)
		default:
			// For complex types, JSON encode
			jsonBytes, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize value for key '%s': %w", k, err)
			}
			value = string(jsonBytes)
		}

		kvs = append(kvs, provider.KeyValue{
			Key:   targetKey,
			Value: value,
		})
	}

	return kvs, nil
}

// parseConfig converts a map[string]interface{} to DopplerConfig
func parseConfig(config map[string]interface{}) (*DopplerConfig, error) {
	// Use JSON marshaling/unmarshaling for clean conversion
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var cfg DopplerConfig
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

