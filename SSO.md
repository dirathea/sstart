# SSO Authentication

sstart supports Single Sign-On (SSO) authentication via OIDC (OpenID Connect). When SSO is configured, sstart will automatically authenticate users before fetching secrets from providers. The obtained tokens can then be used by providers that require OIDC-based authentication (e.g., Vault/OpenBao with JWT auth).

## Authentication Flows

sstart supports two authentication flows:

| Flow | When Used | Requirements | Use Case |
|------|-----------|--------------|----------|
| **Interactive (Browser)** | `SSTART_SSO_SECRET` not set | Client ID only | Local development, user authentication |
| **Client Credentials** | `SSTART_SSO_SECRET` is set | Client ID + Client Secret | CI/CD, automated pipelines, service accounts |

### Interactive Flow (Browser-based)

When no client secret is configured, sstart uses the PKCE (Proof Key for Code Exchange) flow:

1. A local HTTP server starts on port 5747
2. Your default browser opens to the OIDC provider's login page
3. After successful authentication, tokens are cached locally
4. The tokens are used for provider authentication

### Client Credentials Flow (Non-interactive)

When `SSTART_SSO_SECRET` is set, sstart uses the OAuth2 client credentials flow:

1. sstart calls the OIDC provider's token endpoint directly
2. No browser is opened
3. Tokens are obtained automatically
4. Perfect for CI/CD environments

**Important**: If client credentials are configured but authentication fails, sstart will return an error—it will NOT fall back to browser-based authentication.

## Configuration

Add the `sso` section to your `.sstart.yml`:

```yaml
sso:
  oidc:
    clientId: your-client-id        # Required: OIDC client ID
    issuer: https://auth.example.com # Required: OIDC issuer URL
    scopes:                          # Required: OIDC scopes
      - openid
      - profile
      - email

providers:
  - kind: vault
    path: secret/myapp
    auth:
      method: jwt
      role: your-role
```

### Configuration Options

| Field | Required | Description |
|-------|----------|-------------|
| `clientId` | Yes | The OIDC client ID registered with your identity provider |
| `issuer` | Yes | The OIDC issuer URL (e.g., `https://auth.example.com`) |
| `scopes` | Yes | List of OIDC scopes to request. Must include at least one scope. Common scopes: `openid`, `profile`, `email` |
| `pkce` | No | Explicitly enable PKCE flow (`true`/`false`). Defaults to `true` when client secret is not set |
| `redirectUri` | No | Custom redirect URI. Defaults to `http://localhost:5747/auth/sstart` |
| `responseMode` | No | OIDC response mode (e.g., `query`, `fragment`) |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SSTART_SSO_SECRET` | The OIDC client secret. When set, enables client credentials flow (non-interactive). When not set, uses browser-based PKCE flow. |

**Note**: The client secret can ONLY be provided via the `SSTART_SSO_SECRET` environment variable. It is intentionally NOT supported in the YAML config file to prevent accidentally committing secrets to version control.

### Scopes Format

Scopes can be specified as either an array or a space-separated string:

```yaml
# Array format
scopes:
  - openid
  - profile
  - email

# Space-separated string format
scopes: "openid profile email"
```

## Usage Examples

### Interactive Authentication (Local Development)

```yaml
# .sstart.yml
sso:
  oidc:
    clientId: my-public-client
    issuer: https://auth.example.com
    scopes:
      - openid
      - profile
```

```bash
# Just run sstart - browser will open for login
sstart run -- ./my-app
```

### Non-Interactive Authentication (CI/CD)

```yaml
# .sstart.yml (same config - no secret in file!)
sso:
  oidc:
    clientId: my-service-account
    issuer: https://auth.example.com
    scopes:
      - openid
      - profile
```

```bash
# Set the secret via environment variable
export SSTART_SSO_SECRET="your-client-secret"

# sstart will use client credentials flow - no browser
sstart run -- ./my-app
```

### GitHub Actions Example

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run with secrets
        env:
          SSTART_SSO_SECRET: ${{ secrets.OIDC_CLIENT_SECRET }}
        run: |
          sstart run -- ./deploy.sh
```

## Token Storage

sstart stores SSO tokens securely using the system keyring when available, with automatic fallback to file storage.

### Storage Backends

| Backend | Platform | Description |
|---------|----------|-------------|
| **Keyring** (default) | macOS, Windows, Linux | Uses the OS-native secure credential storage |
| **File** (fallback) | All platforms | Falls back to `~/.config/sstart/tokens.json` with 0600 permissions |

#### Keyring Support

- **macOS**: Keychain
- **Windows**: Windows Credential Manager  
- **Linux**: Secret Service (GNOME Keyring, KWallet, etc.)

sstart automatically detects if keyring is available. If not (e.g., in CI/CD environments, headless servers, or containers), it falls back to file-based storage.

### Stored Tokens

The following tokens are stored:

- **Access Token**: Used for authenticating with providers
- **Refresh Token**: Used to obtain new access tokens when expired
- **ID Token**: Contains user identity claims
- **Expiry**: Token expiration timestamp

### Token Refresh

When tokens expire, sstart automatically attempts to refresh them using the refresh token. If refresh fails (e.g., refresh token expired), a new authentication flow is initiated.

## Provider Integration

Providers can access SSO tokens via their configuration to authenticate API requests. The tokens are injected into the provider config with special keys:

| Config Key | Description |
|------------|-------------|
| `_sso_access_token` | The OIDC access token |
| `_sso_id_token` | The OIDC ID token |

Providers that support OIDC authentication can use these tokens to authenticate their API calls. For example, a provider could use the access token as a Bearer token:

```go
// Inside a provider's Fetch implementation
if accessToken, ok := config["_sso_access_token"].(string); ok {
    req.Header.Set("Authorization", "Bearer "+accessToken)
}
```

**Note**: SSO tokens are only used for provider authentication. They are NOT injected as environment variables into the subprocess.

## OIDC Provider Examples

### With Zitadel

```yaml
sso:
  oidc:
    clientId: 351633448147908967
    issuer: https://your-instance.zitadel.cloud
    scopes:
      - openid
      - profile
      - email
```

### With Keycloak

```yaml
sso:
  oidc:
    clientId: sstart-cli
    issuer: https://keycloak.example.com/realms/myrealm
    scopes:
      - openid
      - profile
      - email
```

### With Auth0

```yaml
sso:
  oidc:
    clientId: abc123xyz
    issuer: https://your-tenant.auth0.com
    scopes:
      - openid
      - profile
      - email
```

### With Okta

```yaml
sso:
  oidc:
    clientId: 0oaxxxxxxxx
    issuer: https://your-org.okta.com
    scopes:
      - openid
      - profile
      - email
```

### With Google

```yaml
sso:
  oidc:
    clientId: your-client-id.apps.googleusercontent.com
    issuer: https://accounts.google.com
    scopes:
      - openid
      - profile
      - email
```

### With Azure AD / Entra ID

```yaml
sso:
  oidc:
    clientId: your-application-id
    issuer: https://login.microsoftonline.com/your-tenant-id/v2.0
    scopes:
      - openid
      - profile
      - email
```

## Vault / OpenBao Integration

When using SSO with HashiCorp Vault or OpenBao, sstart can use the OIDC tokens to authenticate with Vault's JWT auth backend. This allows users to access secrets without managing static Vault tokens.

### How It Works

1. User authenticates via OIDC (interactively or via client credentials)
2. sstart obtains an ID token from the OIDC provider
3. The ID token is sent to Vault/OpenBao's JWT auth backend
4. Vault validates the token and returns a Vault token
5. sstart uses the Vault token to fetch secrets

### sstart Configuration

Configure the Vault provider with the `auth` block:

```yaml
sso:
  oidc:
    clientId: your-client-id
    issuer: https://auth.example.com
    scopes:
      - openid
      - profile
      - email

providers:
  - kind: vault
    address: https://vault.example.com
    path: secret/myapp
    auth:
      method: jwt           # or "oidc" - both work the same way
      role: your-vault-role # Required: the JWT auth role in Vault
      mount: jwt            # Optional: auth backend mount path (default: "jwt")
```

#### Configuration Options

| Field | Required | Description |
|-------|----------|-------------|
| `auth.method` | Yes | Set to `oidc` or `jwt` to use SSO tokens for authentication |
| `auth.role` | Yes | The Vault JWT auth role name to authenticate as |
| `auth.mount` | No | The mount path of the JWT auth backend (default: `jwt`) |

### Vault / OpenBao Setup

You need to configure Vault/OpenBao to accept JWT tokens from your OIDC provider.

#### 1. Enable JWT Auth Backend

```bash
# For Vault
vault auth enable jwt

# For OpenBao
bao auth enable jwt
```

#### 2. Configure the JWT Auth Backend

Configure the backend to trust your OIDC provider:

```bash
# For Vault
vault write auth/jwt/config \
  oidc_discovery_url="https://auth.example.com" \
  default_role="sstart"

# For OpenBao
bao write auth/jwt/config \
  oidc_discovery_url="https://auth.example.com" \
  default_role="sstart"
```

Replace `https://auth.example.com` with your OIDC issuer URL.

#### 3. Create a JWT Auth Role

Create a role that maps OIDC users to Vault policies:

```bash
# For Vault
vault write auth/jwt/role/sstart \
  role_type="jwt" \
  bound_audiences="your-client-id" \
  user_claim="sub" \
  policies="your-policy" \
  ttl="1h"

# For OpenBao
bao write auth/jwt/role/sstart \
  role_type="jwt" \
  bound_audiences="your-client-id" \
  user_claim="sub" \
  policies="your-policy" \
  ttl="1h"
```

**Important**: 
- `role_type` must be `jwt` (not `oidc`) because sstart passes the token directly
- `bound_audiences` must match your OIDC client ID exactly

#### 4. Create a Policy

Create a policy that grants access to your secrets:

```bash
# Create policy file
cat > sstart-policy.hcl << EOF
path "secret/data/myapp/*" {
  capabilities = ["read", "list"]
}
EOF

# For Vault
vault policy write sstart-policy sstart-policy.hcl

# For OpenBao
bao policy write sstart-policy sstart-policy.hcl
```

### Example: Complete Setup with Zitadel

#### Zitadel Configuration

1. Create an application in Zitadel:
   - **For interactive use**: Application Type: Native (PKCE enabled)
   - **For CI/CD**: Application Type: Machine-to-machine or Service Account with client credentials
   - Redirect URI: `http://localhost:5747/auth/sstart` (for interactive)

2. Note your Client ID (e.g., `351633448147908967`)

3. For CI/CD: Generate a client secret

#### OpenBao/Vault Configuration

```bash
# Enable JWT auth
bao auth enable jwt

# Configure to trust Zitadel
bao write auth/jwt/config \
  oidc_discovery_url="https://your-instance.zitadel.cloud"

# Create role
bao write auth/jwt/role/sstart \
  role_type="jwt" \
  bound_audiences="351633448147908967" \
  user_claim="sub" \
  policies="sstart-policy" \
  ttl="1h"

# Create policy for reading secrets
bao policy write sstart-policy - << EOF
path "secret/data/*" {
  capabilities = ["read", "list"]
}
EOF
```

#### sstart Configuration

```yaml
sso:
  oidc:
    clientId: 351633448147908967
    issuer: https://your-instance.zitadel.cloud
    scopes:
      - openid
      - profile
      - email

providers:
  - kind: vault
    address: https://vault.example.com
    path: secret/myapp
    auth:
      method: jwt
      role: sstart
```

#### Running

```bash
# Interactive (browser login)
sstart show

# Non-interactive (CI/CD)
export SSTART_SSO_SECRET="your-client-secret"
sstart show
```

## Troubleshooting

### Browser Doesn't Open

If the browser doesn't open automatically, the login URL will be printed to the terminal. Copy and paste it into your browser manually.

```
🔐 Opening browser for authentication...
   If the browser doesn't open, visit: http://localhost:5747/login
```

### Port Already in Use

If port 5747 is already in use, the authentication will fail. Ensure no other application is using this port, or wait for the previous sstart process to complete.

### Token Expired

If you see authentication errors, your tokens may have expired and the refresh token is no longer valid. sstart will automatically initiate a new login flow.

### Clearing Tokens

To force a fresh login, you can use the `--force-auth` flag:

```bash
sstart --force-auth show
```

Or manually clear the stored tokens:

**macOS** (Keychain):
```bash
security delete-generic-password -s sstart -a sso-tokens
```

**Linux** (if using file fallback):
```bash
rm ~/.config/sstart/tokens.json
```

**Windows** (Credential Manager):
Use the Windows Credential Manager UI to remove the "sstart" credential.

### Authentication Timeout

The authentication flow times out after 5 minutes. If you don't complete the login within this time, sstart will fail with a timeout error. Simply run the command again to restart the authentication.

### Client Credentials Flow Fails

If the client credentials flow fails, check:

1. **Client secret is correct**: Verify `SSTART_SSO_SECRET` is set correctly
2. **Grant type enabled**: Ensure your OIDC client has `client_credentials` grant type enabled
3. **Scopes allowed**: Some providers require specific scopes for client credentials

### Vault "permission denied" Error

This usually means the JWT validation failed. Check:

1. **Audience mismatch**: Verify `bound_audiences` matches your OIDC client ID exactly
   ```bash
   bao read auth/jwt/role/sstart
   ```

2. **Issuer not configured**: Verify the OIDC discovery URL is set
   ```bash
   bao read auth/jwt/config
   ```

3. **Role doesn't exist**: Verify the role exists
   ```bash
   bao list auth/jwt/role
   ```

### Vault "role with oidc role_type is not allowed" Error

The role is configured with `role_type="oidc"` but sstart requires `role_type="jwt"`. Update the role:

```bash
bao write auth/jwt/role/sstart \
  role_type="jwt" \
  bound_audiences="your-client-id" \
  user_claim="sub" \
  policies="your-policy" \
  ttl="1h"
```

## Security Considerations

1. **Client Secret via Environment Only**: The client secret can ONLY be provided via `SSTART_SSO_SECRET` environment variable, never in config files. This prevents accidentally committing secrets to version control.

2. **Token Storage**: Tokens are stored in the system keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service) when available. This provides OS-level encryption and access control. Falls back to file storage with restrictive permissions (0600) when keyring is unavailable.

3. **PKCE**: When no client secret is configured, sstart uses PKCE flow for better security in interactive CLI applications.

4. **No Fallback**: When client credentials are configured, sstart will NOT fall back to browser-based authentication if authentication fails. This ensures predictable behavior in CI/CD.

5. **Localhost Callback**: The callback server only binds to `127.0.0.1`, preventing external access.

6. **Session Cookies**: Secure, HTTP-only cookies are used during the authentication flow.

7. **No Token Injection**: SSO tokens are NOT injected into subprocess environment variables, limiting exposure.
