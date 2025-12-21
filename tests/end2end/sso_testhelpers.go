package end2end

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dirathea/sstart/internal/oidc"
)

// GetSSOTestConfig returns the SSO test configuration from environment variables
func GetSSOTestConfig(t *testing.T) (issuer, clientID, clientSecret, idToken, audience string) {
	t.Helper()

	issuer = os.Getenv("SSTART_E2E_SSO_ISSUER")
	if issuer == "" {
		t.Fatalf("SSTART_E2E_SSO_ISSUER environment variable is required")
	}

	clientID = os.Getenv("SSTART_E2E_SSO_CLIENT_ID")
	if clientID == "" {
		t.Fatalf("SSTART_E2E_SSO_CLIENT_ID environment variable is required")
	}

	clientSecret = os.Getenv("SSTART_E2E_SSO_CLIENT_SECRET")
	// Client secret is optional for PKCE-enabled clients

	idToken = os.Getenv("SSTART_E2E_SSO_ID_TOKEN")
	if idToken == "" {
		t.Fatalf("SSTART_E2E_SSO_ID_TOKEN environment variable is required. " +
			"Obtain a token by running: sstart --force-auth show")
	}

	audience = os.Getenv("SSTART_E2E_SSO_AUDIENCE")
	if audience == "" {
		audience = clientID // Default to client ID
	}

	return
}

// SetupSSOTokensFile creates a tokens.json file with the provided ID token
// This allows the OIDC client to find existing tokens and skip browser authentication
// Returns a cleanup function that should be called via defer
func SetupSSOTokensFile(t *testing.T, idToken string) func() {
	t.Helper()

	// Determine the token file path (same logic as in oidc/storage.go)
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get user home directory: %v", err)
		}
		configHome = filepath.Join(homeDir, ".config")
	}

	tokenDir := filepath.Join(configHome, oidc.ConfigDirName)
	tokenPath := filepath.Join(tokenDir, oidc.TokenFileName)

	// Check if tokens file already exists (don't overwrite user's real tokens)
	existingTokens := false
	if _, err := os.Stat(tokenPath); err == nil {
		existingTokens = true
		t.Logf("Existing tokens file found at %s, will restore after test", tokenPath)
	}

	var originalContent []byte
	if existingTokens {
		var err error
		originalContent, err = os.ReadFile(tokenPath)
		if err != nil {
			t.Fatalf("Failed to read existing tokens file: %v", err)
		}
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		t.Fatalf("Failed to create token directory: %v", err)
	}

	// Create tokens with the provided ID token
	// Set expiry to 1 hour from now to ensure it's considered valid
	tokens := &oidc.Tokens{
		IDToken:     idToken,
		AccessToken: "test-access-token", // Dummy access token
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal tokens: %v", err)
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		t.Fatalf("Failed to write tokens file: %v", err)
	}

	t.Logf("Created SSO tokens file at %s for non-interactive testing", tokenPath)

	// Return cleanup function
	return func() {
		if existingTokens {
			// Restore original tokens
			if err := os.WriteFile(tokenPath, originalContent, 0600); err != nil {
				t.Logf("Warning: Failed to restore original tokens file: %v", err)
			}
		} else {
			// Remove the tokens file we created
			if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
				t.Logf("Warning: Failed to remove test tokens file: %v", err)
			}
		}
	}
}

// SetupOpenBaoJWTAuthWithOIDCDiscovery configures JWT auth in OpenBao using OIDC discovery
func SetupOpenBaoJWTAuthWithOIDCDiscovery(ctx context.Context, t *testing.T, container *OpenBaoContainer, issuer, audience, role string, policies []string) {
	t.Helper()

	// Enable JWT auth method
	_, err := container.Client.Logical().Write("sys/auth/jwt", map[string]interface{}{
		"type":        "jwt",
		"description": "JWT auth method for SSO",
	})
	if err != nil {
		// If already enabled, that's okay
		if err.Error() != "path is already in use at jwt/" {
			t.Logf("Note: JWT auth method might already be enabled: %v", err)
		}
	}

	// Configure JWT auth with OIDC discovery
	jwtConfig := map[string]interface{}{
		"oidc_discovery_url": issuer,
		"default_role":       role,
	}
	_, err = container.Client.Logical().Write("auth/jwt/config", jwtConfig)
	if err != nil {
		t.Fatalf("Failed to configure JWT auth with OIDC discovery: %v", err)
	}

	// Create role for JWT auth
	roleConfig := map[string]interface{}{
		"role_type":       "jwt",
		"user_claim":      "sub",
		"policies":        policies,
		"bound_audiences": []string{audience},
	}

	_, err = container.Client.Logical().Write(fmt.Sprintf("auth/jwt/role/%s", role), roleConfig)
	if err != nil {
		t.Fatalf("Failed to create JWT role: %v", err)
	}

	t.Logf("Configured OpenBao JWT auth with OIDC discovery from %s", issuer)
}

// SetupOpenBaoPolicy creates a policy in OpenBao
func SetupOpenBaoPolicy(ctx context.Context, t *testing.T, container *OpenBaoContainer, policyName, policyHCL string) {
	t.Helper()

	_, err := container.Client.Logical().Write(fmt.Sprintf("sys/policies/acl/%s", policyName), map[string]interface{}{
		"policy": policyHCL,
	})
	if err != nil {
		t.Fatalf("Failed to create OpenBao policy: %v", err)
	}
}

