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

		// Create server manager with secrets and inherit flag
		manager := mcp.NewServerManager(serverConfigs, collectedSecrets, cfg.Inherit)

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

func init() {
	rootCmd.AddCommand(mcpCmd)
}
