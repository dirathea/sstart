package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dirathea/sstart/internal/config"
	"github.com/dirathea/sstart/internal/mcp"
	"github.com/dirathea/sstart/internal/secrets"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run as MCP proxy with secret injection",
	Long: `Run sstart as an MCP (Model Context Protocol) proxy server.

This mode allows sstart to act as a proxy between an AI host (like Claude Desktop)
and multiple downstream MCP servers. Secrets from configured providers are 
automatically injected into the downstream servers' environments.

The proxy aggregates tools, resources, and prompts from all downstream servers,
namespacing them with the server ID (e.g., "postgres/query", "filesystem/read_file").

Example configuration (.sstart.yml):
  providers:
    - kind: vault
      address: https://vault.example.com
      path: secret/data/myapp
      keys:
        DATABASE_URL: ==

  mcp:
    servers:
      - id: postgres
        command: npx
        args: ["@modelcontextprotocol/server-postgres"]
      - id: filesystem
        command: npx
        args: ["@modelcontextprotocol/server-filesystem", "/allowed/path"]

Example usage in Claude Desktop config:
  {
    "mcpServers": {
      "secure-servers": {
        "command": "sstart",
        "args": ["mcp", "--config", "/path/to/.sstart.yml"]
      }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle interrupt signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Validate MCP configuration is present
		if !cfg.HasMCP() {
			return fmt.Errorf("mcp configuration not found in config file")
		}

		// Collect secrets from providers
		collector := secrets.NewCollector(cfg, secrets.WithForceAuth(forceAuth))
		collectedSecrets, err := collector.Collect(ctx, providers)
		if err != nil {
			return fmt.Errorf("failed to collect secrets: %w", err)
		}

		// Build environment for downstream servers
		env := buildEnvironment(cfg.Inherit, collectedSecrets)

		// Convert config to MCP server configs
		serverConfigs := make([]mcp.ServerConfig, 0, len(cfg.MCP.Servers))
		for _, s := range cfg.MCP.Servers {
			serverConfig := mcp.ServerConfig{
				ID:      s.ID,
				Command: s.Command,
				Args:    s.Args,
			}
			serverConfigs = append(serverConfigs, serverConfig)
		}

		// Create server manager
		manager := mcp.NewServerManager(serverConfigs, env)

		// Create transport for communication with AI host (stdin/stdout)
		transport := mcp.NewStdioTransport(os.Stdin, os.Stdout)

		// Create and run the proxy
		proxy := mcp.NewProxy(manager, transport)

		// Run proxy (blocks until context is cancelled or EOF)
		err = proxy.Run(ctx)

		// Stop all servers
		proxy.Stop()

		if err != nil && err != context.Canceled {
			return err
		}
		return nil
	},
}

// buildEnvironment builds the environment variable list for downstream servers
func buildEnvironment(inherit bool, secrets map[string]string) []string {
	var env []string

	// Start with system environment if inheriting
	if inherit {
		env = os.Environ()
	}

	// Add collected secrets
	for key, value := range secrets {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
