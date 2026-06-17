#!/usr/bin/env bash
# Regenerate Go clients from the RBHU OpenAPI specs.
# Usage: ./scripts/generate.sh   (run from repo root)
set -euo pipefail

CODEGEN="${CODEGEN:-./bin/oapi-codegen}"
OUT="psd2"

# package_name => spec file
declare -a MAP=(
  "accounts|specs/Hungary_Accounts API Bundle - RBHU_Accounts API.yaml"
  "consent|specs/Hungary_Accounts API Bundle - RBHU_Consent API.yaml"
  "consentauth|specs/Hungary_Accounts API Bundle - RBHU_Consent Authorization API.yaml"
  "cafconsent|specs/Hungary_Confirmation of funds API Bundle - RBHU_BGS Consent CISP API.yaml"
  "caf|specs/Hungary_Confirmation of funds API Bundle - RBHU_Confirmation of funds API.yaml"
  "oauthcisp|specs/Hungary_OAuth2 API Bundle - RBHU_OAuth API CISP.yaml"
  "oauthbridge|specs/Hungary_OAuth2 API Bundle - RBHU_OAuth Bridge API.yaml"
  "oauth2|specs/Hungary_OAuth2 API Bundle - RBHU_OAuth2 API.yaml"
  "bulkpayments|specs/Hungary_Payments API Bundle - RBHU_Bulk Payments API.yaml"
  "cleanuppayments|specs/Hungary_Payments API Bundle - RBHU_Clean-Up Payments API.yaml"
  "payments|specs/Hungary_Payments API Bundle - RBHU_Payment details API.yaml"
  "signingbaskets|specs/Hungary_Payments API Bundle - RBHU_Signing Baskets API.yaml"
  "periodicdomestic|specs/Hungary_Periodic Payments API Bundle - RBHU_Periodic Domestic Payments API.yaml"
  "periodic|specs/Hungary_Periodic Payments API Bundle - RBHU_Periodic Payments API.yaml"
)

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

for entry in "${MAP[@]}"; do
  pkg="${entry%%|*}"
  spec="${entry#*|}"
  dir="$OUT/$pkg"
  mkdir -p "$dir"
  echo "generating $pkg <- $spec"
  clean="$TMP/$pkg.yaml"
  python3 scripts/sanitize_spec.py "$spec" "$clean"
  "$CODEGEN" -generate types,client -package "$pkg" -o "$dir/$pkg.gen.go" "$clean"
done
echo "done"
