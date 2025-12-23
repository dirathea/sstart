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

	issuer = os.Getenv("SSTART_E2E_SSO_ISSUER")
	if issuer == "" {
		t.Fatalf("SSTART_E2E_SSO_ISSUER environment variable is required")
	}

	clientID = os.Getenv("SSTART_E2E_SSO_CLIENT_ID")
	if clientID == "" {
		t.Fatalf("SSTART_E2E_SSO_CLIENT_ID environment variable is required")
	}

	clientSecret = os.Getenv("SSTART_E2E_SSO_CLIENT_SECRET")
	if clientSecret == "" {
		t.Fatalf("SSTART_E2E_SSO_CLIENT_SECRET environment variable is required for client credentials flow")
	}

	audience = os.Getenv("SSTART_E2E_SSO_AUDIENCE")
	if audience == "" {
		audience = clientID // Default to client ID
	}

	return
}

// VerifyOIDCDiscovery checks if the OIDC discovery endpoint is accessible and returns token endpoint
func VerifyOIDCDiscovery(t *testing.T, issuer string) string {
	t.Helper()

	discoveryURL := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(discoveryURL)
	if err != nil {
		t.Fatalf("Failed to fetch OIDC discovery document: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("OIDC discovery returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read OIDC discovery response: %v", err)
	}

	var discovery map[string]interface{}
	if err := json.Unmarshal(body, &discovery); err != nil {
		t.Fatalf("Failed to parse OIDC discovery document: %v", err)
	}

	tokenEndpoint, _ := discovery["token_endpoint"].(string)
	return tokenEndpoint
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
		// If already enabled, that's okay - ignore the error
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
