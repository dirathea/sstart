# OpenBao SSO Development Environment

This directory contains a Docker Compose setup for running OpenBao with SSO (JWT auth) pre-configured for development and manual testing.

## Architecture

The setup uses two containers:

| Container | Purpose |
|-----------|---------|
| `openbao` | The OpenBao server running in dev mode |
| `openbao-manager` | Init container that configures JWT auth, creates policies, roles, and sample secrets |

The manager container waits for OpenBao to be healthy, runs the setup script, then exits. OpenBao remains running with all SSO configuration applied.

## Quick Start

```bash
# Start OpenBao with SSO configured
docker compose up

# Or run in background
docker compose up -d

# View logs
docker compose logs -f

# Check manager setup completed successfully
docker compose logs openbao-manager
```

## What Gets Configured

When `openbao-manager` runs, it automatically:

1. **Enables JWT Auth** - The JWT authentication backend is enabled
2. **Configures OIDC Discovery** - JWT auth is configured to trust your OIDC provider
3. **Creates Policy** - A policy allowing read access to secrets
4. **Creates Role** - A JWT auth role bound to your client ID
5. **Seeds Sample Secrets** - Test secrets are created at:
   - `secret/myapp/config`
   - `secret/myapp/database`
   - `secret/production/credentials`

## Configuration

The setup uses environment variables that can be overridden:

| Variable | Default | Description |
|----------|---------|-------------|
| `OIDC_ISSUER` | `https://sstart-8fucai.us1.zitadel.cloud` | Your OIDC provider URL |
| `OIDC_CLIENT_ID` | `351633448147908967` | Your OIDC client ID |
| `JWT_ROLE` | `sstart` | The JWT auth role name |
| `JWT_POLICY` | `sstart-policy` | The policy name |

### Using Custom Values

Create a `.env` file in this directory:

```bash
# .env
OIDC_ISSUER=https://your-issuer.example.com
OIDC_CLIENT_ID=your-client-id
JWT_ROLE=custom-role
JWT_POLICY=custom-policy
```

Or pass them directly:

```bash
OIDC_CLIENT_ID=my-client-id docker compose up
```

## Testing with sstart

### Example `.sstart.yml`

```yaml
sso:
  oidc:
    clientId: <client id>
    issuer: <issuer url>
    scopes:
      - openid
      - profile
      - email

providers:
  - kind: vault
    address: http://localhost:8200
    path: myapp/config
    auth:
      method: jwt
      role: sstart
```

### Run sstart

```bash
# Interactive (browser login)
sstart show

# With client credentials (CI/CD)
SSTART_SSO_SECRET=your-client-secret sstart show
```

## Manual Verification

You can verify the setup using the OpenBao CLI:

```bash
# Check JWT auth config
docker compose exec openbao bao read auth/jwt/config

# Check the role
docker compose exec openbao bao read auth/jwt/role/sstart

# Check the policy
docker compose exec openbao bao policy read sstart-policy

# List secrets
docker compose exec openbao bao kv list secret/

# Read a secret
docker compose exec openbao bao kv get secret/myapp/config
```

## Accessing the UI

OpenBao UI is available at: http://localhost:8200/ui

- **Token**: `openbao`

## Cleanup

```bash
# Stop and remove containers
docker compose down

# Remove volumes too
docker compose down -v
```

## Troubleshooting

### JWT Auth Not Working

1. Verify OIDC discovery URL is accessible:
   ```bash
   curl https://your-issuer/.well-known/openid-configuration
   ```

2. Check that client ID matches your OIDC application's client ID

3. Verify the role is configured correctly:
   ```bash
   docker compose exec openbao bao read auth/jwt/role/sstart
   ```

### Permission Denied

Make sure your OIDC token has the correct audience claim that matches `bound_audiences` in the role configuration.

