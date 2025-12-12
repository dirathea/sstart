package end2end

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// DopplerClient wraps HTTP client and API configuration for Doppler
type DopplerClient struct {
	client    *http.Client
	apiHost   string
	authToken string
}

// DopplerSecretsUpdateRequest represents the request body for updating secrets
// According to Doppler API: https://docs.doppler.com/reference/secrets-update
// The body format is:
//
//	{
//	  "project": "PROJECT_NAME",
//	  "config": "CONFIG_NAME",
//	  "secrets": {
//	    "SECRET_NAME": "secret_value"
//	  }
//	}
type DopplerSecretsUpdateRequest struct {
	Project string            `json:"project"`
	Config  string            `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

// SetupDopplerClient creates and authenticates a Doppler client for testing
func SetupDopplerClient(ctx context.Context, t *testing.T) *DopplerClient {
	t.Helper()

	// Check for required environment variable
	authToken := os.Getenv("DOPPLER_TOKEN")
	if authToken == "" {
		t.Skipf("Skipping test: DOPPLER_TOKEN environment variable is required")
	}

	// Get API host from environment variable (optional, defaults to https://api.doppler.com)
	apiHost := os.Getenv("DOPPLER_API_HOST")
	if apiHost == "" {
		apiHost = "https://api.doppler.com"
	}

	return &DopplerClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiHost:   apiHost,
		authToken: authToken,
	}
}

// GetDopplerTestProject returns the test project name from environment variable
func GetDopplerTestProject(t *testing.T) string {
	t.Helper()

	project := os.Getenv("SSTART_E2E_DOPPLER_PROJECT")
	if project == "" {
		t.Skipf("Skipping test: SSTART_E2E_DOPPLER_PROJECT environment variable is required")
	}

	return project
}

// GetDopplerTestConfig returns the test config/environment name from environment variable
func GetDopplerTestConfig(t *testing.T) string {
	t.Helper()

	config := os.Getenv("SSTART_E2E_DOPPLER_CONFIG")
	if config == "" {
		t.Skipf("Skipping test: SSTART_E2E_DOPPLER_CONFIG environment variable is required")
	}

	return config
}

// SetupDopplerSecret creates or updates a secret in Doppler for testing
func SetupDopplerSecret(ctx context.Context, t *testing.T, client *DopplerClient, project, config, secretName, secretValue string) {
	t.Helper()

	// Build API URL for updating a secret
	apiURL := fmt.Sprintf("%s/v3/configs/config/secrets", client.apiHost)

	// Create request body according to Doppler API format:
	// {
	//   "project": "PROJECT_NAME",
	//   "config": "CONFIG_NAME",
	//   "secrets": {
	//     "SECRET_NAME": "secret_value"
	//   }
	// }
	updateReq := DopplerSecretsUpdateRequest{
		Project: project,
		Config:  config,
		Secrets: map[string]string{
			secretName: secretValue,
		},
	}

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("Failed to marshal secret update request: %v", err)
	}

	// Create HTTP request
	// According to Doppler API docs: https://docs.doppler.com/reference/secrets-update
	// The endpoint uses POST method
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.authToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Make HTTP request
	resp, err := client.client.Do(req)
	if err != nil {
		t.Fatalf("Failed to update secret in Doppler: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Doppler API returned status %d: %s", resp.StatusCode, string(body))
	}
}

// SetupDopplerSecretsBatch creates or updates multiple secrets in Doppler
// secrets is a map of secretName -> secretValue
func SetupDopplerSecretsBatch(ctx context.Context, t *testing.T, client *DopplerClient, project, config string, secrets map[string]string) {
	t.Helper()

	if len(secrets) == 0 {
		return
	}

	// Build API URL for updating secrets
	apiURL := fmt.Sprintf("%s/v3/configs/config/secrets", client.apiHost)

	// Build request body according to Doppler API format:
	// {
	//   "project": "PROJECT_NAME",
	//   "config": "CONFIG_NAME",
	//   "secrets": {
	//     "SECRET_NAME": "secret_value"
	//   }
	// }
	updateReq := DopplerSecretsUpdateRequest{
		Project: project,
		Config:  config,
		Secrets: secrets,
	}

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("Failed to marshal secrets update request: %v", err)
	}

	// Create HTTP request
	// According to Doppler API docs: https://docs.doppler.com/reference/secrets-update
	// The endpoint uses POST method
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.authToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Make HTTP request
	resp, err := client.client.Do(req)
	if err != nil {
		t.Fatalf("Failed to update secrets in Doppler: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Doppler API returned status %d: %s", resp.StatusCode, string(body))
	}
}

// DeleteDopplerSecret deletes a secret from Doppler using the DELETE endpoint
// According to Doppler API: https://docs.doppler.com/reference/configs-config-secret-delete
func DeleteDopplerSecret(ctx context.Context, t *testing.T, client *DopplerClient, project, config, secretName string) {
	t.Helper()

	// Build API URL for deleting a secret
	// Endpoint: /v3/configs/config/secret?project=PROJECT_NAME&config=CONFIG_NAME&name=SECRET_NAME
	apiURL := fmt.Sprintf("%s/v3/configs/config/secret?project=%s&config=%s&name=%s",
		client.apiHost, url.QueryEscape(project), url.QueryEscape(config), url.QueryEscape(secretName))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		t.Logf("Note: Could not create delete request for secret '%s': %v", secretName, err)
		return
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.authToken))
	req.Header.Set("Accept", "application/json")

	// Make HTTP request
	resp, err := client.client.Do(req)
	if err != nil {
		t.Logf("Note: Could not delete secret '%s' from Doppler: %v", secretName, err)
		return
	}
	defer resp.Body.Close()

	// Log but don't fail - the secret might not exist, which is fine
	// Accept both 200 OK and 204 No Content as success responses
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Note: Could not delete secret '%s' from Doppler (status %d): %s", secretName, resp.StatusCode, string(body))
	}
}

// DeleteDopplerSecretsBatch deletes multiple secrets from Doppler (if they exist)
// secretNames is a slice of secret names to delete
func DeleteDopplerSecretsBatch(ctx context.Context, t *testing.T, client *DopplerClient, project, config string, secretNames []string) {
	t.Helper()

	if len(secretNames) == 0 {
		return
	}

	// Delete each secret individually
	for _, secretName := range secretNames {
		DeleteDopplerSecret(ctx, t, client, project, config, secretName)
	}
}

// VerifyDopplerSecretExists checks if a secret exists in Doppler
func VerifyDopplerSecretExists(ctx context.Context, t *testing.T, client *DopplerClient, project, config, secretName string) {
	t.Helper()

	// Build API URL for downloading secrets
	apiURL := fmt.Sprintf("%s/v3/configs/config/secrets/download?format=json&project=%s&config=%s",
		client.apiHost, project, config)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.authToken))
	req.Header.Set("Accept", "application/json")

	// Make HTTP request
	resp, err := client.client.Do(req)
	if err != nil {
		t.Fatalf("Failed to fetch secrets from Doppler: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Skipf("Skipping test: Failed to fetch secrets from Doppler (status %d): %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Parse JSON response
	var secretData map[string]interface{}
	if err := json.Unmarshal(body, &secretData); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check if secret exists
	if _, exists := secretData[secretName]; !exists {
		t.Skipf("Skipping test: Secret '%s' does not exist in project '%s' config '%s'. "+
			"Please create it beforehand in your Doppler project.", secretName, project, config)
	}
}
