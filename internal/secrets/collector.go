package secrets

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/dirathea/sstart/internal/config"
	"github.com/dirathea/sstart/internal/provider"
)

// Collector collects secrets from all configured providers
type Collector struct {
	config *config.Config
}

// NewCollector creates a new secrets collector
func NewCollector(cfg *config.Config) *Collector {
	return &Collector{config: cfg}
}

// Collect fetches secrets from all providers and combines them
func (c *Collector) Collect(ctx context.Context, providerIDs []string) (map[string]string, error) {
	secrets := make(map[string]string)

	// If no providers specified, use all providers in order
	if len(providerIDs) == 0 {
		for _, provider := range c.config.Providers {
			providerIDs = append(providerIDs, provider.ID)
		}
	}

	// Collect from each provider
	for _, providerID := range providerIDs {
		providerCfg, err := c.config.GetProvider(providerID)
		if err != nil {
			return nil, err
		}

		// Create provider instance
		prov, err := provider.New(providerCfg.Kind)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider '%s': %w", providerID, err)
		}

		// Expand template variables in config (e.g., in path fields)
		expandedConfig := expandConfigTemplates(providerCfg.Config)

		// Fetch secrets from this provider's single source
		kvs, err := prov.Fetch(ctx, providerCfg.ID, expandedConfig, providerCfg.Keys)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch from provider '%s': %w", providerID, err)
		}

		// Merge secrets (later providers override earlier ones)
		for _, kv := range kvs {
			secrets[kv.Key] = kv.Value
		}
	}

	return secrets, nil
}

// expandConfigTemplates expands template variables in config values
// Supports {{ get_env(name="VAR", default="default") }} syntax
func expandConfigTemplates(config map[string]interface{}) map[string]interface{} {
	expanded := make(map[string]interface{})
	for k, v := range config {
		switch val := v.(type) {
		case string:
			expanded[k] = expandTemplate(val)
		case map[string]interface{}:
			expanded[k] = expandConfigTemplates(val)
		case []interface{}:
			expandedSlice := make([]interface{}, len(val))
			for i, item := range val {
				if str, ok := item.(string); ok {
					expandedSlice[i] = expandTemplate(str)
				} else {
					expandedSlice[i] = item
				}
			}
			expanded[k] = expandedSlice
		default:
			expanded[k] = v
		}
	}
	return expanded
}

// expandTemplate expands template variables in a string
// Supports {{ get_env(name="VAR", default="default") }} syntax
func expandTemplate(template string) string {
	// Simple implementation: expand environment variables
	re := regexp.MustCompile(`\{\{\s*get_env\(name="([^"]+)",\s*default="([^"]+)"\)\s*\}\}`)
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) == 3 {
			envVar := matches[1]
			defaultValue := matches[2]
			if value := os.Getenv(envVar); value != "" {
				return value
			}
			return defaultValue
		}
		return match
	})

	// Also support simple ${VAR} or $VAR syntax
	result = os.ExpandEnv(result)

	return result
}

// Redact redacts secrets from text
func Redact(text string, secrets map[string]string) string {
	result := text
	for _, value := range secrets {
		if len(value) > 0 {
			// Redact the full value
			mask := strings.Repeat("*", len(value))
			result = strings.ReplaceAll(result, value, mask)
		}
	}
	return result
}

// Mask masks a secret value, showing only first and last characters
func Mask(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	if len(value) <= 8 {
		return value[:2] + "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}
