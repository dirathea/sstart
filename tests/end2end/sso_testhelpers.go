package end2end

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// maskString returns a masked version of the string for safe logging
func maskString(s string, showChars int) string {
	if len(s) <= showChars*2 {
		return strings.Repeat("*", len(s))
	}
	return s[:showChars] + strings.Repeat("*", len(s)-showChars*2) + s[len(s)-showChars:]
}

// GetSSOTestConfig returns the SSO test configuration from environment variables
// Required env vars:
//   - SSTART_E2E_SSO_ISSUER: OIDC issuer URL
//   - SSTART_E2E_SSO_CLIENT_ID: OIDC client ID
//   - SSTART_E2E_SSO_CLIENT_SECRET: OIDC client secret (for client credentials flow)
//
// Optional env vars:
//   - SSTART_E2E_SSO_AUDIENCE: Expected audience (defaults to client ID)
func GetSSOTestConfig(t *testing.T) (issuer, clientID, clientSecret, audience string) {
	t.Helper()

	t.Log("=== SSO Test Configuration ===")

	issuer = os.Getenv("SSTART_E2E_SSO_ISSUER")
	if issuer == "" {
		t.Fatalf("SSTART_E2E_SSO_ISSUER environment variable is required")
	}
	t.Logf("  Issuer: %s", issuer)

	clientID = os.Getenv("SSTART_E2E_SSO_CLIENT_ID")
	if clientID == "" {
		t.Fatalf("SSTART_E2E_SSO_CLIENT_ID environment variable is required")
	}
	t.Logf("  Client ID: %s (length: %d)", maskString(clientID, 4), len(clientID))

	clientSecret = os.Getenv("SSTART_E2E_SSO_CLIENT_SECRET")
	if clientSecret == "" {
		t.Fatalf("SSTART_E2E_SSO_CLIENT_SECRET environment variable is required for client credentials flow")
	}
	t.Logf("  Client Secret: %s (length: %d)", maskString(clientSecret, 4), len(clientSecret))

	audience = os.Getenv("SSTART_E2E_SSO_AUDIENCE")
	if audience == "" {
		audience = clientID // Default to client ID
		t.Logf("  Audience: (defaulting to client ID)")
	} else {
		t.Logf("  Audience: %s", maskString(audience, 4))
	}

	t.Log("=== End SSO Test Configuration ===")

	return
}

// VerifyOIDCDiscovery checks if the OIDC discovery endpoint is accessible and returns token endpoint
func VerifyOIDCDiscovery(t *testing.T, issuer string) string {
	t.Helper()

	discoveryURL := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	t.Logf("Verifying OIDC discovery at: %s", discoveryURL)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(discoveryURL)
	if err != nil {
		t.Logf("WARNING: Failed to fetch OIDC discovery document: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("WARNING: OIDC discovery returned status %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Logf("WARNING: Failed to read OIDC discovery response: %v", err)
		return ""
	}

	var discovery map[string]interface{}
	if err := json.Unmarshal(body, &discovery); err != nil {
		t.Logf("WARNING: Failed to parse OIDC discovery document: %v", err)
		return ""
	}

	// Log important endpoints
	t.Log("=== OIDC Discovery Document ===")
	if iss, ok := discovery["issuer"].(string); ok {
		t.Logf("  issuer: %s", iss)
	}
	tokenEndpoint := ""
	if te, ok := discovery["token_endpoint"].(string); ok {
		tokenEndpoint = te
		t.Logf("  token_endpoint: %s", te)
	}
	if jwks, ok := discovery["jwks_uri"].(string); ok {
		t.Logf("  jwks_uri: %s", jwks)
	}
	if grants, ok := discovery["grant_types_supported"].([]interface{}); ok {
		t.Logf("  grant_types_supported: %v", grants)
	}
	t.Log("=== End OIDC Discovery Document ===")

	return tokenEndpoint
}

// SetupOpenBaoJWTAuthWithOIDCDiscovery configures JWT auth in OpenBao using OIDC discovery
func SetupOpenBaoJWTAuthWithOIDCDiscovery(ctx context.Context, t *testing.T, container *OpenBaoContainer, issuer, audience, role string, policies []string) {
	t.Helper()

	t.Log("=== Setting up OpenBao JWT Auth ===")
	t.Logf("  OpenBao Address: %s", container.Address)
	t.Logf("  OIDC Issuer: %s", issuer)
	t.Logf("  Audience: %s", audience)
	t.Logf("  Role: %s", role)
	t.Logf("  Policies: %v", policies)

	// Enable JWT auth method
	t.Log("Enabling JWT auth method...")
	_, err := container.Client.Logical().Write("sys/auth/jwt", map[string]interface{}{
		"type":        "jwt",
		"description": "JWT auth method for SSO",
	})
	if err != nil {
		// If already enabled, that's okay
		if err.Error() != "path is already in use at jwt/" {
			t.Logf("Note: JWT auth method might already be enabled: %v", err)
		}
	} else {
		t.Log("  JWT auth method enabled successfully")
	}

	// Configure JWT auth with OIDC discovery
	t.Log("Configuring JWT auth with OIDC discovery...")
	jwtConfig := map[string]interface{}{
		"oidc_discovery_url": issuer,
		"default_role":       role,
	}
	_, err = container.Client.Logical().Write("auth/jwt/config", jwtConfig)
	if err != nil {
		t.Fatalf("Failed to configure JWT auth with OIDC discovery: %v", err)
	}
	t.Log("  JWT auth configured successfully")

	// Read back and verify the JWT config
	t.Log("Verifying JWT auth configuration...")
	configSecret, err := container.Client.Logical().Read("auth/jwt/config")
	if err != nil {
		t.Logf("WARNING: Failed to read back JWT config: %v", err)
	} else if configSecret != nil && configSecret.Data != nil {
		t.Logf("  Configured oidc_discovery_url: %v", configSecret.Data["oidc_discovery_url"])
		t.Logf("  Configured default_role: %v", configSecret.Data["default_role"])
	}

	// Create role for JWT auth
	t.Logf("Creating JWT role '%s'...", role)
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
	t.Log("  JWT role created successfully")

	// Read back and verify the role
	t.Log("Verifying JWT role configuration...")
	roleSecret, err := container.Client.Logical().Read(fmt.Sprintf("auth/jwt/role/%s", role))
	if err != nil {
		t.Logf("WARNING: Failed to read back JWT role: %v", err)
	} else if roleSecret != nil && roleSecret.Data != nil {
		t.Logf("  Role type: %v", roleSecret.Data["role_type"])
		t.Logf("  User claim: %v", roleSecret.Data["user_claim"])
		t.Logf("  Policies: %v", roleSecret.Data["policies"])
		t.Logf("  Bound audiences: %v", roleSecret.Data["bound_audiences"])
	}

	t.Log("=== OpenBao JWT Auth Setup Complete ===")
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
