package end2end

import (
	"context"
	"fmt"
	"os"
	"testing"
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

