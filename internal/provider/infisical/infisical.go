package infisical

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dirathea/sstart/internal/provider"
	infisical "github.com/infisical/go-sdk"
)

// InfisicalConfig represents the configuration for Infisical provider
type InfisicalConfig struct {
	// ProjectID is the Infisical project ID (required)
	ProjectID string `json:"project_id" yaml:"project_id"`
	// Environment is the environment slug (e.g., dev, prod) (required)
	Environment string `json:"environment" yaml:"environment"`
	// Path is the secret path from where to fetch secrets (required)
	Path string `json:"path" yaml:"path"`
	// Recursive indicates whether to fetch secrets recursively from subdirectories (optional, default: false)
	Recursive *bool `json:"recursive,omitempty" yaml:"recursive,omitempty"`
	// IncludeImports specifies whether to include imported secrets (optional, default: false)
	IncludeImports *bool `json:"include_imports,omitempty" yaml:"include_imports,omitempty"`
	// ExpandSecrets determines whether to expand secret references (optional, default: false)
	ExpandSecrets *bool `json:"expand_secrets,omitempty" yaml:"expand_secrets,omitempty"`
}

// InfisicalProvider implements the provider interface for Infisical
type InfisicalProvider struct {
	client infisical.InfisicalClientInterface
}

func init() {
	provider.Register("infisical", func() provider.Provider {
		return &InfisicalProvider{}
	})
}

// Name returns the provider name
func (p *InfisicalProvider) Name() string {
	return "infisical"
}

// Fetch fetches secrets from Infisical
func (p *InfisicalProvider) Fetch(ctx context.Context, mapID string, config map[string]interface{}, keys map[string]string) ([]provider.KeyValue, error) {
	// Convert map to strongly typed config struct
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid infisical configuration: %w", err)
	}

	// Validate required fields
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("infisical provider requires 'project_id' field in configuration")
	}
	if cfg.Environment == "" {
		return nil, fmt.Errorf("infisical provider requires 'environment' field in configuration")
	}
	if cfg.Path == "" {
		return nil, fmt.Errorf("infisical provider requires 'path' field in configuration")
	}

	// Ensure client is initialized
	if err := p.ensureClient(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Infisical client: %w", err)
	}

	// Set default values for optional parameters
	recursive := false
	if cfg.Recursive != nil {
		recursive = *cfg.Recursive
	}

	includeImports := false
	if cfg.IncludeImports != nil {
		includeImports = *cfg.IncludeImports
	}

	expandSecrets := false
	if cfg.ExpandSecrets != nil {
		expandSecrets = *cfg.ExpandSecrets
	}

	// Build ListSecretsOptions
	listOptions := infisical.ListSecretsOptions{
		ProjectID:              cfg.ProjectID,
		Environment:            cfg.Environment,
		SecretPath:             cfg.Path,
		Recursive:              recursive,
		IncludeImports:         includeImports,
		ExpandSecretReferences: expandSecrets,
	}

	// Fetch secrets using the SDK
	secrets, err := p.client.Secrets().List(listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets from Infisical: %w", err)
	}

	// Convert secrets to key-value pairs
	secretData := make(map[string]interface{})
	for _, secret := range secrets {
		// Use the secret key as the key, and the secret value as the value
		secretData[secret.SecretKey] = secret.SecretValue
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

		value := fmt.Sprintf("%v", v)
		kvs = append(kvs, provider.KeyValue{
			Key:   targetKey,
			Value: value,
		})
	}

	return kvs, nil
}

// ensureClient initializes the Infisical client if not already initialized
func (p *InfisicalProvider) ensureClient(ctx context.Context) error {
	if p.client != nil {
		return nil
	}

	log.Printf("[Infisical] Initializing Infisical client...")

	// Check for required environment variables
	clientID := os.Getenv("INFISICAL_UNIVERSAL_AUTH_CLIENT_ID")
	clientSecret := os.Getenv("INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Printf("[Infisical] ERROR: Missing required environment variables")
		log.Printf("[Infisical] INFISICAL_UNIVERSAL_AUTH_CLIENT_ID is set: %v", clientID != "")
		log.Printf("[Infisical] INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET is set: %v", clientSecret != "")
		return fmt.Errorf("INFISICAL_UNIVERSAL_AUTH_CLIENT_ID and INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET environment variables are required")
	}

	log.Printf("[Infisical] Found INFISICAL_UNIVERSAL_AUTH_CLIENT_ID (length: %d)", len(clientID))
	log.Printf("[Infisical] Found INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET (length: %d)", len(clientSecret))

	// Get site URL from environment variable (optional, defaults to https://app.infisical.com)
	siteURL := os.Getenv("INFISICAL_SITE_URL")

	// Create client config
	clientConfig := infisical.Config{}
	if siteURL != "" {
		clientConfig.SiteUrl = siteURL
		log.Printf("[Infisical] Using custom site URL: %s", siteURL)
	} else {
		log.Printf("[Infisical] Using default site URL: https://app.infisical.com")
	}

	// Create client with config
	log.Printf("[Infisical] Creating Infisical client...")
	client := infisical.NewInfisicalClient(ctx, clientConfig)

	// Authenticate using universal auth (pass env vars as parameters)
	log.Printf("[Infisical] Attempting Universal Auth login to: %s", clientConfig.SiteUrl)
	credential, err := client.Auth().UniversalAuthLogin(clientID, clientSecret)
	if err != nil {
		log.Printf("[Infisical] ERROR: Authentication failed")
		log.Printf("[Infisical]   Error: %v", err)
		log.Printf("[Infisical]   Site URL: %s", clientConfig.SiteUrl)
		log.Printf("[Infisical]   Client ID length: %d", len(clientID))
		if len(clientID) > 8 {
			log.Printf("[Infisical]   Client ID prefix: %s...", clientID[:8])
		} else {
			log.Printf("[Infisical]   Client ID: %s", clientID)
		}
		log.Printf("[Infisical]   Client Secret length: %d", len(clientSecret))
		if len(clientSecret) > 8 {
			log.Printf("[Infisical]   Client Secret prefix: %s...", clientSecret[:8])
		} else {
			log.Printf("[Infisical]   Client Secret: %s", clientSecret)
		}
		return fmt.Errorf("failed to authenticate with Infisical (401 Unauthorized): %w", err)
	}

	// Log successful authentication (check if AccessToken field exists)
	if credential.AccessToken != "" {
		log.Printf("[Infisical] Successfully authenticated (Access Token length: %d)", len(credential.AccessToken))
	} else {
		log.Printf("[Infisical] Successfully authenticated")
	}
	p.client = client
	return nil
}

// parseConfig converts a map[string]interface{} to InfisicalConfig
func parseConfig(config map[string]interface{}) (*InfisicalConfig, error) {
	// Use JSON marshaling/unmarshaling for clean conversion
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var cfg InfisicalConfig
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
