#!/usr/bin/env bash
# Run the HTTP connector locally and expose it over HTTPS for a claude.ai
# custom connector. Prints the bearer token and the public /mcp URL.
# Usage: ./scripts/connector-dev.sh   (or: make connector-dev)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ADDR="${RBHU_CONNECTOR_ADDR:-127.0.0.1:8080}"
TOKEN="${RBHU_CONNECTOR_TOKEN:-$(openssl rand -hex 32)}"

echo "==> building bin/rbhu-connector-http"
go build -o bin/rbhu-connector-http ./cmd/rbhu-connector-http

echo "==> starting connector on http://$ADDR (MCP at /mcp)"
RBHU_CONNECTOR_TOKEN="$TOKEN" RBHU_CONNECTOR_ADDR="$ADDR" ./bin/rbhu-connector-http &
SRV=$!
trap 'kill $SRV 2>/dev/null || true' EXIT

cat <<EOF

  Bearer token (configure this in the claude.ai connector):
    $TOKEN

EOF

if command -v cloudflared >/dev/null 2>&1; then
  echo "==> opening cloudflared tunnel (public HTTPS URL below; add <url>/mcp in claude.ai)"
  cloudflared tunnel --url "http://$ADDR"
elif command -v ngrok >/dev/null 2>&1; then
  echo "==> opening ngrok tunnel (use the https forwarding URL + /mcp in claude.ai)"
  ngrok http "$ADDR"
else
  echo "No tunnel tool found (install 'cloudflared' or 'ngrok')."
  echo "Connector is running locally at http://$ADDR/mcp — expose it over HTTPS yourself."
  wait $SRV
fi
