package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/dirathea/sstart/internal/provider"
)

// TemplateConfig represents the configuration for template provider
type TemplateConfig struct {
	// Templates is a map of template expressions using dot notation: PG_URI: pgsql://{{.aws_prod.PG_USERNAME}}:{{.aws_prod.PG_PASSWORD}}@{{.aws_generic.PG_HOST}}
	Templates map[string]string `yaml:"templates"`
}

// TemplateProvider implements the provider interface for template-based secret manipulation
type TemplateProvider struct{}

func init() {
	provider.Register("template", func() provider.Provider {
		return &TemplateProvider{}
	})
}

// Name returns the provider name
func (p *TemplateProvider) Name() string {
	return "template"
}

// parseConfig converts a map[string]interface{} to TemplateConfig
func parseConfig(config map[string]interface{}) (*TemplateConfig, error) {
	// Use JSON marshaling/unmarshaling for clean conversion
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var cfg TemplateConfig
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// Fetch fetches secrets by resolving template expressions
// The templates map contains template expressions using dot notation: PG_URI: pgsql://{{.aws_prod.PG_USERNAME}}:{{.aws_prod.PG_PASSWORD}}@{{.aws_generic.PG_HOST}}
func (p *TemplateProvider) Fetch(secretContext provider.SecretContext, mapID string, config map[string]interface{}, keys map[string]string) ([]provider.KeyValue, error) {
	// Get SecretsResolver from secretContext
	resolver := secretContext.SecretsResolver
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid template configuration: %w", err)
	}

	// Get templates from config
	if len(cfg.Templates) == 0 {
		return nil, fmt.Errorf("template provider requires 'templates' field with template expressions")
	}

	// Resolve each template expression
	kvs := make([]provider.KeyValue, 0, len(cfg.Templates))
	for targetKey, templateExpr := range cfg.Templates {
		resolvedValue, err := p.resolveTemplate(templateExpr, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve template for key '%s': %w", targetKey, err)
		}
		kvs = append(kvs, provider.KeyValue{
			Key:   targetKey,
			Value: resolvedValue,
		})
	}

	return kvs, nil
}

// resolveTemplate resolves a template expression using Go's text/template package
// Template syntax: {{.provider_id.secret_key}} (dot notation, similar to Helm templates)
// Example: {{.aws_prod.PG_USERNAME}} or {{.aws_generic.PG_HOST}}
func (p *TemplateProvider) resolveTemplate(templateStr string, resolver provider.SecretsResolver) (string, error) {
	// Build template data structure from resolver
	// Structure: { "provider_id": { "secret_key": "value", ... }, ... }
	providerSecrets := resolver.Map()

	// Parse the template
	tmpl, err := template.New("secret_template").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute the template with the data structure
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, providerSecrets); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
