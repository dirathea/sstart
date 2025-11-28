package end2end

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/hashicorp/vault/api"
	localstack "github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/vault"
)

// LocalStackContainer wraps LocalStack container and its endpoint
type LocalStackContainer struct {
	Container *localstack.LocalStackContainer
	Endpoint  string
	Cleanup   func() error
}

// VaultContainer wraps Vault container, address, and client
type VaultContainer struct {
	Container *vault.VaultContainer
	Address   string
	Client    *api.Client
	Cleanup   func() error
}

// SetupLocalStack starts a LocalStack container and returns the container info
func SetupLocalStack(ctx context.Context, t *testing.T) *LocalStackContainer {
	t.Helper()

	container, err := localstack.Run(ctx, "localstack/localstack:latest")
	if err != nil {
		t.Fatalf("Failed to start localstack container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get localstack host: %v", err)
	}

	port, err := container.MappedPort(ctx, "4566/tcp")
	if err != nil {
		t.Fatalf("Failed to get localstack port: %v", err)
	}

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &LocalStackContainer{
		Container: container,
		Endpoint:  endpoint,
		Cleanup: func() error {
			return container.Terminate(ctx)
		},
	}
}

// SetupVault starts a Vault container and returns the container info
func SetupVault(ctx context.Context, t *testing.T) *VaultContainer {
	t.Helper()

	container, err := vault.Run(ctx, "hashicorp/vault:latest",
		vault.WithToken("test-token"),
	)
	if err != nil {
		t.Fatalf("Failed to start vault container: %v", err)
	}

	address, err := container.HttpHostAddress(ctx)
	if err != nil {
		t.Fatalf("Failed to get vault address: %v", err)
	}

	client, err := api.NewClient(&api.Config{
		Address: address,
	})
	if err != nil {
		t.Fatalf("Failed to create vault client: %v", err)
	}
	client.SetToken("test-token")

	return &VaultContainer{
		Container: container,
		Address:   address,
		Client:    client,
		Cleanup: func() error {
			return container.Terminate(ctx)
		},
	}
}

// SetupContainers sets up both LocalStack and Vault containers
func SetupContainers(ctx context.Context, t *testing.T) (*LocalStackContainer, *VaultContainer) {
	t.Helper()

	localstack := SetupLocalStack(ctx, t)
	vault := SetupVault(ctx, t)

	// Wait for containers to be ready
	time.Sleep(2 * time.Second)

	return localstack, vault
}

// SetupAWSSecret creates a secret in AWS Secrets Manager (LocalStack)
func SetupAWSSecret(ctx context.Context, t *testing.T, localstack *LocalStackContainer, secretName string, secretData map[string]string) {
	t.Helper()

	awsRegion := "us-east-1"
	secretJSON, err := json.Marshal(secretData)
	if err != nil {
		t.Fatalf("Failed to marshal secret data: %v", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(awsRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("Failed to load AWS config: %v", err)
	}

	secretsManagerClient := secretsmanager.NewFromConfig(awsCfg, func(o *secretsmanager.Options) {
		o.BaseEndpoint = aws.String(localstack.Endpoint)
	})

	_, err = secretsManagerClient.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(string(secretJSON)),
	})
	if err != nil {
		t.Fatalf("Failed to create secret in AWS Secrets Manager: %v", err)
	}
}

// SetupVaultSecret enables KV v2 engine (if needed) and writes a secret to Vault
func SetupVaultSecret(ctx context.Context, t *testing.T, vaultContainer *VaultContainer, vaultPath string, secretData map[string]interface{}) {
	t.Helper()

	// Enable KV v2 secrets engine (if not already enabled)
	_, err := vaultContainer.Client.Logical().Write("sys/mounts/secret", map[string]interface{}{
		"type":        "kv-v2",
		"description": "KV v2 secrets engine",
	})
	if err != nil {
		// If error is that path is already in use, that's okay
		if !strings.Contains(err.Error(), "path is already in use") {
			t.Fatalf("Failed to enable KV v2 secrets engine: %v", err)
		}
	}

	_, err = vaultContainer.Client.Logical().Write(fmt.Sprintf("secret/data/%s", vaultPath), map[string]interface{}{
		"data": secretData,
	})
	if err != nil {
		t.Fatalf("Failed to write secret to Vault: %v", err)
	}
}
