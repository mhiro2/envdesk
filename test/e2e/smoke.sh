#!/usr/bin/env bash
# E2E smoke tests - basic happy path validation
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/age-identity.sh
source "${SCRIPT_DIR}/lib/age-identity.sh"

ROOT="$(mktemp -d)"
BIN="${GITHUB_WORKSPACE:-$(pwd)}/bin/envdesk"

cleanup() {
  rm -rf "$ROOT"
}
trap cleanup EXIT

echo "=== E2E Smoke Tests ==="
echo "Working directory: $ROOT"
echo ""

# Setup age key for SOPS encryption
cd "$ROOT"
git init -q

e2e_setup_age_identity

echo "--- Test: init ---"
"$BIN" init --services api --envs dev,stg --sops --encrypt --age "$RECIPIENT"

# Verify init created expected files
test -f envdesk.yaml
test -f .sops.yaml
test -f env/api/dev.env
test -f env/api/stg.env
echo "PASS: init created expected files"

echo "--- Test: lint (encrypted envs) ---"
"$BIN" lint
echo "PASS: lint succeeded"

echo "--- Test: diff ---"
# Without --ci-summary, exit 0 even when dev/stg differ; still fails on real errors.
"$BIN" diff api dev stg
echo "PASS: diff executed"

echo "--- Test: export ---"
"$BIN" export api dev --out "$ROOT/.env.local"
test -f "$ROOT/.env.local"
echo "PASS: export created .env.local"

echo "--- Test: export content validation ---"
# Verify exported content is valid env format
grep -q '=' "$ROOT/.env.local" || {
  echo "FAIL: exported file should contain key=value pairs"
  exit 1
}
echo "PASS: export content is valid"

echo "--- Test: doctor ---"
"$BIN" doctor --json
echo "PASS: doctor succeeded"

echo ""
echo "=== All smoke tests passed ==="
