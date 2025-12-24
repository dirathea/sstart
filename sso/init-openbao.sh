#!/bin/sh
set -e

# BAO_TOKEN and BAO_ADDR are set via environment variables from docker-compose

echo "🔧 Configuring OpenBao with SSO (JWT Auth)..."
echo "   OIDC Issuer: ${OIDC_ISSUER}"
echo "   OIDC Client ID: ${OIDC_CLIENT_ID}"
echo "   JWT Role: ${JWT_ROLE}"
echo "   JWT Policy: ${JWT_POLICY}"

# ============================================
# 1. Enable JWT Auth Method
# ============================================
echo ""
echo "📝 Enabling JWT auth method..."
bao auth enable jwt 2>/dev/null || echo "   (JWT auth already enabled)"

# ============================================
# 2. Configure JWT Auth with OIDC Discovery
# ============================================
echo "📝 Configuring JWT auth with OIDC discovery..."
bao write auth/jwt/config \
  oidc_discovery_url="${OIDC_ISSUER}" \
  default_role="${JWT_ROLE}"

# ============================================
# 3. Create Policy for Reading Secrets
# ============================================
echo "📝 Creating policy '${JWT_POLICY}'..."
bao policy write ${JWT_POLICY} - <<EOF
# Allow reading secrets from the secret/ path
path "secret/data/*" {
  capabilities = ["read", "list"]
}

path "secret/metadata/*" {
  capabilities = ["read", "list"]
}

# Allow listing secrets
path "secret/*" {
  capabilities = ["list"]
}
EOF

# ============================================
# 4. Create JWT Auth Role
# ============================================
echo "📝 Creating JWT role '${JWT_ROLE}'..."
bao write auth/jwt/role/${JWT_ROLE} \
  role_type="jwt" \
  bound_audiences="${OIDC_CLIENT_ID}" \
  user_claim="sub" \
  policies="${JWT_POLICY}" \
  ttl="1h"

# ============================================
# 5. Create Sample Secrets (for testing)
# ============================================
echo "📝 Creating sample secrets..."

# Enable KV v2 secrets engine (might already be enabled in dev mode)
bao secrets enable -path=secret kv-v2 2>/dev/null || echo "   (KV v2 already enabled)"

# Sample secrets for testing
bao kv put secret/myapp/config \
  API_KEY="test-api-key-12345" \
  DB_PASSWORD="super-secret-password" \
  DEBUG="false"

bao kv put secret/myapp/database \
  HOST="localhost" \
  PORT="5432" \
  USER="admin" \
  PASSWORD="db-secret-password"

bao kv put secret/production/credentials \
  AWS_ACCESS_KEY="AKIAIOSFODNN7EXAMPLE" \
  AWS_SECRET_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

# ============================================
# Summary
# ============================================
echo ""
echo "============================================"
echo "✅ OpenBao SSO Setup Complete!"
echo "============================================"
echo ""
echo "📍 OpenBao Address: http://localhost:8200"
echo "🔑 Root Token: openbao"
echo ""
echo "🔐 JWT Auth Configuration:"
echo "   - OIDC Issuer: ${OIDC_ISSUER}"
echo "   - Client ID (audience): ${OIDC_CLIENT_ID}"
echo "   - Role: ${JWT_ROLE}"
echo "   - Policy: ${JWT_POLICY}"
echo ""
echo "📦 Sample Secrets Created:"
echo "   - secret/myapp/config"
echo "   - secret/myapp/database"
echo "   - secret/production/credentials"
echo ""
echo "📝 Example .sstart.yml:"
echo ""
echo "sso:"
echo "  oidc:"
echo "    clientId: ${OIDC_CLIENT_ID}"
echo "    issuer: ${OIDC_ISSUER}"
echo "    scopes:"
echo "      - openid"
echo "      - profile"
echo "      - email"
echo ""
echo "providers:"
echo "  - kind: vault"
echo "    address: http://localhost:8200"
echo "    path: myapp/config"
echo "    auth:"
echo "      method: jwt"
echo "      role: ${JWT_ROLE}"
echo ""
echo "============================================"

