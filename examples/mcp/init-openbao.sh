#!/bin/sh
set -e

echo "==> Initializing OpenBao with MongoDB credentials..."

# Wait for OpenBao to be ready
until wget --spider --quiet http://openbao:8200/v1/sys/health 2>/dev/null; do
  echo "Waiting for OpenBao..."
  sleep 1
done

# Enable KV v2 secrets engine
bao secrets enable -path=secret kv-v2 2>/dev/null || echo "KV engine already enabled"

# Store MongoDB credentials separately
bao kv put secret/mongodb \
  username="admin" \
  password="secret123" \
  host="localhost" \
  port="27017" \
  database="demo" \
  auth_database="admin"

echo "==> OpenBao initialized successfully!"
echo ""
echo "MongoDB credentials stored at: secret/mongodb"
echo "  username: admin"
echo "  password: [hidden]"
echo "  host: localhost"
echo "  port: 27017"
echo "  database: demo"
echo "  auth_database: admin"
echo ""
echo "You can verify with:"
echo "  docker exec sstart-demo-openbao bao kv get -address=http://localhost:8200 -token=demo-token secret/mongodb"
