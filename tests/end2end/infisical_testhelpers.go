package end2end

import (
	"context"
	"os"
	"testing"

	infisical "github.com/infisical/go-sdk"
)

// SetupInfisicalClient creates and authenticates an Infisical client for testing
func SetupInfisicalClient(ctx context.Context, t *testing.T) infisical.InfisicalClientInterface {
	t.Helper()

	t.Log("[Infisical E2E] Initializing Infisical client for testing...")

	// Check for required environment variables
	clientID := os.Getenv("INFISICAL_UNIVERSAL_AUTH_CLIENT_ID")
	clientSecret := os.Getenv("INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Logf("[Infisical E2E] ERROR: Missing required environment variables")
		t.Logf("[Infisical E2E] INFISICAL_UNIVERSAL_AUTH_CLIENT_ID is set: %v", clientID != "")
		t.Logf("[Infisical E2E] INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET is set: %v", clientSecret != "")
		t.Skipf("Skipping test: INFISICAL_UNIVERSAL_AUTH_CLIENT_ID and INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET environment variables are required")
	}

	t.Logf("[Infisical E2E] Found INFISICAL_UNIVERSAL_AUTH_CLIENT_ID (length: %d)", len(clientID))
	t.Logf("[Infisical E2E] Found INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET (length: %d)", len(clientSecret))

	// Get site URL from environment variable (optional, defaults to https://app.infisical.com)
	siteURL := os.Getenv("INFISICAL_SITE_URL")

	// Create client config
	clientConfig := infisical.Config{}
	if siteURL != "" {
		clientConfig.SiteUrl = siteURL
		t.Logf("[Infisical E2E] Using custom site URL: %s", siteURL)
	} else {
		t.Logf("[Infisical E2E] Using default site URL: https://app.infisical.com")
	}

	// Create client
	t.Log("[Infisical E2E] Creating Infisical client...")
	client := infisical.NewInfisicalClient(ctx, clientConfig)

	// Authenticate using universal auth
	t.Logf("[Infisical E2E] Attempting Universal Auth login to: %s", clientConfig.SiteUrl)
	credential, err := client.Auth().UniversalAuthLogin(clientID, clientSecret)
	if err != nil {
		t.Logf("[Infisical E2E] ERROR: Authentication failed")
		t.Logf("[Infisical E2E]   Error: %v", err)
		t.Logf("[Infisical E2E]   Site URL: %s", clientConfig.SiteUrl)
		t.Logf("[Infisical E2E]   Client ID length: %d", len(clientID))
		if len(clientID) > 8 {
			t.Logf("[Infisical E2E]   Client ID prefix: %s...", clientID[:8])
		} else {
			t.Logf("[Infisical E2E]   Client ID: %s", clientID)
		}
		t.Logf("[Infisical E2E]   Client Secret length: %d", len(clientSecret))
		if len(clientSecret) > 8 {
			t.Logf("[Infisical E2E]   Client Secret prefix: %s...", clientSecret[:8])
		} else {
			t.Logf("[Infisical E2E]   Client Secret: %s", clientSecret)
		}
		t.Fatalf("Failed to authenticate with Infisical: %v", err)
	}

	// Log successful authentication
	if credential.AccessToken != "" {
		t.Logf("[Infisical E2E] Successfully authenticated (Access Token length: %d)", len(credential.AccessToken))
	} else {
		t.Log("[Infisical E2E] Successfully authenticated")
	}

	return client
}

// GetInfisicalTestProjectID returns the test project ID from environment variable
func GetInfisicalTestProjectID(t *testing.T) string {
	t.Helper()

	t.Log("[Infisical E2E] Reading test project ID from environment...")
	projectID := os.Getenv("SSTART_E2E_INFISICAL_PROJECT_ID")
	if projectID == "" {
		t.Log("[Infisical E2E] ERROR: SSTART_E2E_INFISICAL_PROJECT_ID environment variable is not set")
		t.Skipf("Skipping test: SSTART_E2E_INFISICAL_PROJECT_ID environment variable is required")
	}

	t.Logf("[Infisical E2E] Found test project ID (length: %d)", len(projectID))
	return projectID
}

// GetInfisicalTestEnvironment returns the test environment from environment variable
func GetInfisicalTestEnvironment(t *testing.T) string {
	t.Helper()

	t.Log("[Infisical E2E] Reading test environment from environment...")
	environment := os.Getenv("SSTART_E2E_INFISICAL_ENVIRONMENT")
	if environment == "" {
		t.Log("[Infisical E2E] ERROR: SSTART_E2E_INFISICAL_ENVIRONMENT environment variable is not set")
		t.Skipf("Skipping test: SSTART_E2E_INFISICAL_ENVIRONMENT environment variable is required")
	}

	t.Logf("[Infisical E2E] Found test environment: %s", environment)
	return environment
}

// EnsureInfisicalPathExists ensures that the given path exists in Infisical
// For root path "/", this is a no-op as it always exists
// For other paths, Infisical will automatically create the path structure when secrets are created
func EnsureInfisicalPathExists(ctx context.Context, t *testing.T, client infisical.InfisicalClientInterface, projectID, environment, secretPath string) {
	t.Helper()

	// Root path always exists, no need to check or create
	if secretPath == "/" || secretPath == "" {
		t.Logf("[Infisical E2E] Path is root (/), always exists")
		return
	}

	t.Logf("[Infisical E2E] Ensuring path exists: %s", secretPath)

	// Note: Infisical automatically creates folder paths when secrets are created at those paths
	// So we don't need to explicitly create folders here - the path will be created
	// automatically when we create a secret at this path
	t.Logf("[Infisical E2E] Path will be created automatically when secrets are added to this path")
}

// SetupInfisicalSecret creates or updates a secret in Infisical for testing
// It ensures the path exists and then creates/updates the secret without listing all secrets first
func SetupInfisicalSecret(ctx context.Context, t *testing.T, client infisical.InfisicalClientInterface, projectID, environment, secretPath, secretKey, secretValue string) {
	t.Helper()

	t.Logf("[Infisical E2E] Setting up secret: %s at path: %s", secretKey, secretPath)
	t.Logf("[Infisical E2E]   Project ID: %s", projectID)
	t.Logf("[Infisical E2E]   Environment: %s", environment)

	// Ensure the path exists
	EnsureInfisicalPathExists(ctx, t, client, projectID, environment, secretPath)

	// Try to create the secret first
	t.Logf("[Infisical E2E] Attempting to create secret: %s", secretKey)
	createOptions := infisical.CreateSecretOptions{
		SecretKey:   secretKey,
		ProjectID:   projectID,
		Environment: environment,
		SecretPath:  secretPath,
		SecretValue: secretValue,
	}

	_, err := client.Secrets().Create(createOptions)
	if err != nil {
		// If creation fails, it might be because the secret already exists
		// Try to update it instead
		t.Logf("[Infisical E2E] Create failed (secret may already exist), attempting to update: %v", err)

		updateOptions := infisical.UpdateSecretOptions{
			SecretKey:   secretKey,
			ProjectID:   projectID,
			Environment: environment,
			SecretPath:  secretPath,
		}
		updateOptions.NewSecretValue = secretValue

		_, err := client.Secrets().Update(updateOptions)
		if err != nil {
			t.Logf("[Infisical E2E] ERROR: Failed to update secret: %v", err)
			t.Fatalf("Failed to create or update secret in Infisical: %v", err)
		}
		t.Logf("[Infisical E2E] Successfully updated secret: %s", secretKey)
	} else {
		t.Logf("[Infisical E2E] Successfully created secret: %s", secretKey)
	}
}

// VerifyInfisicalSecretExists checks if a secret exists in Infisical
func VerifyInfisicalSecretExists(ctx context.Context, t *testing.T, client infisical.InfisicalClientInterface, projectID, environment, secretPath, secretKey string) {
	t.Helper()

	t.Logf("[Infisical E2E] Verifying secret exists: %s at path: %s", secretKey, secretPath)
	t.Logf("[Infisical E2E]   Project ID: %s", projectID)
	t.Logf("[Infisical E2E]   Environment: %s", environment)

	listOptions := infisical.ListSecretsOptions{
		ProjectID:   projectID,
		Environment: environment,
		SecretPath:  secretPath,
	}

	t.Logf("[Infisical E2E] Listing secrets at path: %s", secretPath)
	secrets, err := client.Secrets().List(listOptions)
	if err != nil {
		t.Logf("[Infisical E2E] ERROR: Failed to list secrets: %v", err)
		t.Fatalf("Failed to list secrets from Infisical: %v", err)
	}

	t.Logf("[Infisical E2E] Found %d secrets at path", len(secrets))

	for _, secret := range secrets {
		if secret.SecretKey == secretKey && secret.SecretPath == secretPath {
			t.Logf("[Infisical E2E] Secret '%s' verified to exist", secretKey)
			return // Secret exists
		}
	}

	t.Logf("[Infisical E2E] ERROR: Secret '%s' not found at path '%s'", secretKey, secretPath)
	t.Skipf("Skipping test: Secret '%s' does not exist at path '%s' in environment '%s' of project '%s'. "+
		"Please create it beforehand in your Infisical project.", secretKey, secretPath, environment, projectID)
}
