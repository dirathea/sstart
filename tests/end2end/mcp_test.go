package end2end

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MCPMessage represents a JSON-RPC 2.0 MCP message
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// MCPInitializeResult represents the result of an initialize request
type MCPInitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Tools     interface{} `json:"tools,omitempty"`
		Resources interface{} `json:"resources,omitempty"`
		Prompts   interface{} `json:"prompts,omitempty"`
	} `json:"capabilities"`
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// MCPToolsListResult represents the result of tools/list
type MCPToolsListResult struct {
	Tools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		InputSchema json.RawMessage `json:"inputSchema"`
	} `json:"tools"`
}

// createMockMCPServer creates a simple mock MCP server script
func createMockMCPServer(t *testing.T, dir string, serverID string, tools []string) string {
	t.Helper()

	// Create a simple shell script that acts as an MCP server
	// It responds to initialize and tools/list
	scriptContent := fmt.Sprintf(`#!/bin/bash
# Mock MCP Server: %s
# Reads JSON-RPC messages from stdin and responds

while IFS= read -r line; do
    # Parse the method from the JSON (simple grep approach)
    method=$(echo "$line" | grep -o '"method":"[^"]*"' | cut -d'"' -f4)
    id=$(echo "$line" | grep -o '"id":[0-9]*' | cut -d':' -f2)
    
    if [ -z "$id" ]; then
        id=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    fi
    
    case "$method" in
        "initialize")
            echo '{"jsonrpc":"2.0","id":'$id',"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"%s","version":"1.0.0"}}}'
            ;;
        "notifications/initialized")
            # No response for notifications
            ;;
        "tools/list")
            # Return the tools for this server
            tools_json='[%s]'
            echo '{"jsonrpc":"2.0","id":'$id',"result":{"tools":'$tools_json'}}'
            ;;
        "tools/call")
            # Echo back the call with DATABASE_URL from env
            db_url="${DATABASE_URL:-not_set}"
            echo '{"jsonrpc":"2.0","id":'$id',"result":{"content":[{"type":"text","text":"DATABASE_URL='$db_url'"}]}}'
            ;;
        "ping")
            echo '{"jsonrpc":"2.0","id":'$id',"result":{}}'
            ;;
        *)
            if [ -n "$method" ]; then
                echo '{"jsonrpc":"2.0","id":'$id',"error":{"code":-32601,"message":"Method not found"}}'
            fi
            ;;
    esac
done
`, serverID, serverID, strings.Join(tools, ","))

	scriptPath := filepath.Join(dir, fmt.Sprintf("mock_mcp_%s.sh", serverID))
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock MCP server script: %v", err)
	}

	return scriptPath
}

// TestE2E_MCP_ConfigValidation tests MCP configuration validation
func TestE2E_MCP_ConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a dummy .env file for valid config tests
	dummyEnvPath := filepath.Join(tmpDir, ".env.dummy")
	if err := os.WriteFile(dummyEnvPath, []byte("DUMMY=value\n"), 0644); err != nil {
		t.Fatalf("Failed to create dummy .env: %v", err)
	}

	tests := []struct {
		name      string
		config    string
		wantError string
	}{
		{
			name: "valid config",
			config: fmt.Sprintf(`
providers:
  - kind: dotenv
    path: %s

mcp:
  servers:
    - id: test
      command: echo
      args: ["hello"]
`, dummyEnvPath),
			wantError: "",
		},
		{
			name: "missing server id",
			config: `
mcp:
  servers:
    - command: echo
`,
			wantError: "id is required",
		},
		{
			name: "missing server command",
			config: `
mcp:
  servers:
    - id: test
`,
			wantError: "command is required",
		},
		{
			name: "duplicate server id",
			config: `
mcp:
  servers:
    - id: test
      command: echo
    - id: test
      command: echo
`,
			wantError: "duplicate",
		},
		{
			name: "empty servers list",
			config: `
mcp:
  servers: []
`,
			wantError: "at least one server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, fmt.Sprintf("config_%s.yml", strings.ReplaceAll(tt.name, " ", "_")))
			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}

			// Try to run sstart mcp with the config
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "go", "run", "../../cmd/sstart", "mcp", "--config", configPath)
			output, err := cmd.CombinedOutput()

			if tt.wantError == "" {
				// For valid config, we expect it to start (and eventually timeout or fail due to no real servers)
				// but not fail config validation
				if err != nil && strings.Contains(string(output), tt.wantError) {
					t.Errorf("Unexpected config validation error: %s", output)
				}
			} else {
				// For invalid config, we expect an error containing our expected string
				if err == nil || !strings.Contains(strings.ToLower(string(output)), strings.ToLower(tt.wantError)) {
					t.Errorf("Expected error containing '%s', got: %s", tt.wantError, output)
				}
			}
		})
	}
}

// buildSstart builds the sstart binary for testing
func buildSstart(t *testing.T, tmpDir string) string {
	t.Helper()
	binaryPath := filepath.Join(tmpDir, "sstart")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/sstart")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build sstart: %v\n%s", err, output)
	}
	return binaryPath
}

// TestE2E_MCP_ProxyBasic tests basic MCP proxy functionality with mock servers
func TestE2E_MCP_ProxyBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	tmpDir := t.TempDir()
	binaryPath := buildSstart(t, tmpDir)

	// Create mock MCP server
	toolJSON := `{"name":"test_tool","description":"A test tool","inputSchema":{"type":"object"}}`
	serverScript := createMockMCPServer(t, tmpDir, "mockserver", []string{toolJSON})

	// Create .env file with test secret
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte("DATABASE_URL=postgres://test:test@localhost:5432/testdb\n"), 0644); err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Create config
	config := fmt.Sprintf(`
providers:
  - kind: dotenv
    path: %s

mcp:
  servers:
    - id: mockserver
      command: bash
      args: ["%s"]
`, envPath, serverScript)

	configPath := filepath.Join(tmpDir, ".sstart.yml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Start sstart mcp
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "mcp", "--config", configPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sstart mcp: %v", err)
	}

	// Give it a moment to start
	time.Sleep(500 * time.Millisecond)

	// Create a scanner to read responses
	scanner := bufio.NewScanner(stdout)

	// Send initialize request
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`
	if _, err := io.WriteString(stdin, initReq+"\n"); err != nil {
		t.Fatalf("Failed to send initialize: %v", err)
	}

	// Read response with timeout
	responseCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		if scanner.Scan() {
			responseCh <- scanner.Text()
		} else {
			errCh <- scanner.Err()
		}
	}()

	select {
	case response := <-responseCh:
		var initResp MCPMessage
		if err := json.Unmarshal([]byte(response), &initResp); err != nil {
			t.Fatalf("Failed to parse initialize response: %v", err)
		}

		if initResp.Error != nil {
			t.Fatalf("Initialize failed: %s", initResp.Error.Message)
		}

		var initResult MCPInitializeResult
		if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
			t.Fatalf("Failed to parse initialize result: %v", err)
		}

		if initResult.ServerInfo.Name != "sstart-mcp-proxy" {
			t.Errorf("Expected server name 'sstart-mcp-proxy', got '%s'", initResult.ServerInfo.Name)
		}
	case err := <-errCh:
		t.Fatalf("Failed to read initialize response: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for initialize response")
	}

	// Send initialized notification
	if _, err := io.WriteString(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"); err != nil {
		t.Fatalf("Failed to send initialized notification: %v", err)
	}

	// Send tools/list request
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	if _, err := io.WriteString(stdin, toolsReq+"\n"); err != nil {
		t.Fatalf("Failed to send tools/list: %v", err)
	}

	// Read tools response with timeout
	go func() {
		if scanner.Scan() {
			responseCh <- scanner.Text()
		} else {
			errCh <- scanner.Err()
		}
	}()

	select {
	case response := <-responseCh:
		var toolsResp MCPMessage
		if err := json.Unmarshal([]byte(response), &toolsResp); err != nil {
			t.Fatalf("Failed to parse tools/list response: %v", err)
		}

		if toolsResp.Error != nil {
			t.Fatalf("tools/list failed: %s", toolsResp.Error.Message)
		}

		var toolsResult MCPToolsListResult
		if err := json.Unmarshal(toolsResp.Result, &toolsResult); err != nil {
			t.Fatalf("Failed to parse tools result: %v", err)
		}

		// Should have namespaced tools (mockserver/test_tool)
		if len(toolsResult.Tools) == 0 {
			t.Error("Expected at least one tool")
		} else {
			foundNamespacedTool := false
			for _, tool := range toolsResult.Tools {
				if tool.Name == "mockserver/test_tool" {
					foundNamespacedTool = true
					break
				}
			}
			if !foundNamespacedTool {
				t.Errorf("Expected tool 'mockserver/test_tool', got tools: %+v", toolsResult.Tools)
			}
		}
	case err := <-errCh:
		t.Fatalf("Failed to read tools/list response: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for tools/list response")
	}

	// Close stdin to signal we're done
	stdin.Close()

	// Wait for process to exit
	cmd.Wait()
}

// TestE2E_MCP_SecretInjection tests that secrets are injected into MCP servers
func TestE2E_MCP_SecretInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	tmpDir := t.TempDir()

	// Create mock MCP server that echoes DATABASE_URL
	toolJSON := `{"name":"echo_env","description":"Echo environment","inputSchema":{"type":"object"}}`
	serverScript := createMockMCPServer(t, tmpDir, "envserver", []string{toolJSON})

	// Create .env file with test secret
	testDBURL := "postgres://secret:password@localhost:5432/secretdb"
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(fmt.Sprintf("DATABASE_URL=%s\n", testDBURL)), 0644); err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Create config
	config := fmt.Sprintf(`
providers:
  - kind: dotenv
    path: %s

mcp:
  servers:
    - id: envserver
      command: bash
      args: ["%s"]
`, envPath, serverScript)

	configPath := filepath.Join(tmpDir, ".sstart.yml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Start sstart mcp
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "../../cmd/sstart", "mcp", "--config", configPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sstart mcp: %v", err)
	}

	time.Sleep(2 * time.Second)

	scanner := bufio.NewScanner(stdout)

	// Initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	io.WriteString(stdin, initReq+"\n")
	scanner.Scan()

	// Send initialized notification
	io.WriteString(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")

	// Call the tool to check if DATABASE_URL is injected
	toolCallReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"envserver/echo_env","arguments":{}}}`
	io.WriteString(stdin, toolCallReq+"\n")

	if !scanner.Scan() {
		t.Fatalf("Failed to read tools/call response: %v", scanner.Err())
	}

	var toolCallResp MCPMessage
	if err := json.Unmarshal(scanner.Bytes(), &toolCallResp); err != nil {
		t.Fatalf("Failed to parse tools/call response: %v", err)
	}

	if toolCallResp.Error != nil {
		t.Fatalf("tools/call failed: %s", toolCallResp.Error.Message)
	}

	// Check that the response contains the DATABASE_URL
	resultStr := string(toolCallResp.Result)
	if !strings.Contains(resultStr, testDBURL) {
		t.Errorf("Expected DATABASE_URL to be injected, got: %s", resultStr)
	}

	stdin.Close()
	cmd.Wait()
}
