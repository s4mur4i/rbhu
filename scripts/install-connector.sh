#!/usr/bin/env bash
# Build the AIS connector and register it with Claude Code (user scope).
# Usage: ./scripts/install-connector.sh   (or: make install)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> building bin/rbhu-connector"
go build -o bin/rbhu-connector ./cmd/rbhu-connector

ENV_FILE="$ROOT/secrets/.env"
P12="$ROOT/secrets/certificate_RBHU_SB_KONG_PROD.p12"

if [ ! -f "$ENV_FILE" ]; then
  echo "WARNING: $ENV_FILE not found — copy .env.example to secrets/.env and fill it in."
fi

if ! command -v claude >/dev/null 2>&1; then
  cat <<EOF
Claude Code CLI ('claude') not found. Register manually:

  claude mcp add --scope user rbhu-ais \\
    --env RBHU_ENV_FILE="$ENV_FILE" \\
    --env RBHU_P12="$P12" \\
    -- "$ROOT/bin/rbhu-connector"
EOF
  exit 0
fi

echo "==> registering 'rbhu-ais' with Claude Code (user scope)"
claude mcp remove -s user rbhu-ais >/dev/null 2>&1 || true
claude mcp add --scope user rbhu-ais \
  --env RBHU_ENV_FILE="$ENV_FILE" \
  --env RBHU_P12="$P12" \
  -- "$ROOT/bin/rbhu-connector"

echo "==> done. Restart Claude Code (or run /mcp -> reconnect) to load 'rbhu-ais'."
echo "    (Inside this repo you can instead just approve the bundled .mcp.json — no need for this command.)"
