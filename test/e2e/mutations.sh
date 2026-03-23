#!/usr/bin/env bash
# E2E mutation tests - edit, sync-keys, example generate
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/age-identity.sh
source "${SCRIPT_DIR}/lib/age-identity.sh"
ROOT="$(mktemp -d)"
BIN="${GITHUB_WORKSPACE:-$(pwd)}/bin/envdesk"
FAKE_EDITOR="$SCRIPT_DIR/lib/fake-editor.sh"

cleanup() {
  rm -rf "$ROOT"
}
trap cleanup EXIT

echo "=== E2E Mutation Tests ==="
echo "Working directory: $ROOT"
echo ""

# Setup
cd "$ROOT"
git init -q

e2e_setup_age_identity

# Initialize with encryption
"$BIN" init --services api --envs dev,stg,prod --sops --encrypt --age "$RECIPIENT"

echo "--- Test: edit adds new key ---"
export EDITOR="$FAKE_EDITOR"
"$BIN" edit api dev --no-lint

# Verify the key was added by exporting and checking
"$BIN" export api dev --out "$ROOT/check-edit.env"
grep -q 'NEW_KEY=fake_editor_added' "$ROOT/check-edit.env" || {
  echo "FAIL: edit should have added NEW_KEY"
  exit 1
}
echo "PASS: edit added new key"

echo "--- Test: sync-keys propagates keys ---"
# Get current stg content and verify it doesn't have NEW_KEY
"$BIN" export api stg --out "$ROOT/stg-before.env"
if grep -q 'NEW_KEY' "$ROOT/stg-before.env"; then
  echo "INFO: stg already has NEW_KEY (from init)"
fi

# Sync keys from dev to stg (dry-run first)
"$BIN" sync-keys api dev --to stg --dry-run --placeholders
echo "PASS: sync-keys dry-run succeeded"

# Apply sync
"$BIN" sync-keys api dev --to stg --placeholders

# Verify NEW_KEY is now in stg
"$BIN" export api stg --out "$ROOT/check-sync.env"
grep -q 'NEW_KEY' "$ROOT/check-sync.env" || {
  echo "FAIL: sync-keys should have added NEW_KEY to stg"
  exit 1
}
echo "PASS: sync-keys propagated keys"

echo "--- Test: example generate ---"
"$BIN" example generate --service api --force
test -f env/api/.env.example || {
  echo "FAIL: example generate should create .env.example"
  exit 1
}
echo "PASS: example generate created .env.example"

echo ""
echo "=== All mutation tests passed ==="
