package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dirathea/sstart/internal/config"
	"github.com/dirathea/sstart/internal/secrets"
	"github.com/spf13/cobra"
)

var envFormat string

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Export secrets in environment variable format",
	Long: `Export secrets in a format suitable for --env-file or shell export.

Example:
  docker run --env-file <(sstart env) alpine sh
  eval "$(sstart env)"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Collect secrets
		collector := secrets.NewCollector(cfg)
		envProviders := providers
		if len(envProviders) == 0 {
			envProviders = nil // Use all providers
		}
		envSecrets, err := collector.Collect(ctx, envProviders)
		if err != nil {
			return fmt.Errorf("failed to collect secrets: %w", err)
		}

		// Export in requested format
		switch envFormat {
		case "json":
			jsonBytes, err := json.MarshalIndent(envSecrets, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonBytes))
		case "yaml":
			for key, value := range envSecrets {
				fmt.Printf("%s: %s\n", key, escapeYAML(value))
			}
		default: // shell format
			for key, value := range envSecrets {
				fmt.Printf("export %s=%s\n", key, escapeShell(value))
			}
		}

		return nil
	},
}

func escapeShell(s string) string {
	// Escape single quotes by ending the quoted string, escaping the quote, and restarting
	s = strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + s + "'"
}

func escapeYAML(s string) string {
	// For YAML, quote if contains special characters
	if strings.ContainsAny(s, ":{}[],&*#?|-<>=!%@`") || strings.Contains(s, "\n") {
		// Use double quotes and escape double quotes and backslashes
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		return "\"" + s + "\""
	}
	return s
}

func init() {
	envCmd.Flags().StringVar(&envFormat, "format", "shell", "Output format: shell, json, or yaml")
	envCmd.Flags().StringSliceVar(&providers, "providers", []string{}, "Comma-separated list of provider IDs to use (default: all providers)")
	rootCmd.AddCommand(envCmd)
}
