#!/usr/bin/env bash
# E2E failure tests - verify error handling for invalid scenarios
set -euo pipefail

ROOT="$(mktemp -d)"
BIN="${GITHUB_WORKSPACE:-$(pwd)}/bin/envdesk"

cleanup() {
  rm -rf "$ROOT"
}
trap cleanup EXIT

echo "=== E2E Failure Tests ==="
echo "Working directory: $ROOT"
echo ""

# Setup
cd "$ROOT"
git init -q

age-keygen -o age.txt >/dev/null 2>&1
RECIPIENT="$(grep '^# public key:' age.txt | sed 's/# public key: //')"
export SOPS_AGE_KEY_FILE="$ROOT/age.txt"

echo "--- Test: missing sops config detection ---"
# Create project without sops
"$BIN" init --services api --envs dev --force

# Try to use encrypted operations without .sops.yaml
rm -f .sops.yaml

if "$BIN" doctor 2>/dev/null; then
  echo "FAIL: doctor should fail when .sops.yaml is missing"
  exit 1
fi
echo "PASS: doctor detects missing .sops.yaml"

echo "--- Test: schema violation detection ---"
# Reset with proper sops setup
"$BIN" init --services api --envs dev,stg --sops --encrypt --age "$RECIPIENT" --force

# Replace scaffold schema: require DATABASE_URL while init only provides APP_ENV.
cat > env.schema/api.yaml <<'EOF'
keys:
  DATABASE_URL:
    required: true
    type: string
    secret: true
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
EOF

# Fake editor appends an undeclared key; lint must still fail on missing DATABASE_URL.
cat > "$ROOT/fake-editor-missing.sh" <<'INNEREOF'
#!/usr/bin/env bash
echo "MISSING_KEY=not_added" >> "$1"
INNEREOF
chmod +x "$ROOT/fake-editor-missing.sh"

# Edit succeeds with --no-lint; lint must then report schema violations.
export EDITOR="$ROOT/fake-editor-missing.sh"
"$BIN" edit api dev --no-lint

if "$BIN" lint 2>/dev/null; then
  echo "FAIL: lint should fail when required keys are missing"
  exit 1
fi
echo "PASS: lint detects missing required keys"

echo "--- Test: drift detection ---"
# Create stg with different keys using edit
cat > "$ROOT/fake-editor-stg.sh" <<'INNEREOF'
#!/usr/bin/env bash
echo "EXTRA_KEY=extra" >> "$1"
INNEREOF
chmod +x "$ROOT/fake-editor-stg.sh"

export EDITOR="$ROOT/fake-editor-stg.sh"
"$BIN" edit api stg --no-lint

if "$BIN" check-sync --ci-summary 2>/dev/null; then
  echo "FAIL: check-sync should detect drift"
  exit 1
fi
echo "PASS: check-sync detects key drift"

echo "--- Test: tracked plaintext detection ---"
# Reset to encrypted mode
"$BIN" init --services api --envs prod --sops --encrypt --age "$RECIPIENT" --force

# Create a plaintext env file and track it
cat > env/api/plaintext.env <<'EOF'
SECRET_VALUE=exposed
EOF
git add env/api/plaintext.env 2>/dev/null || true

set +e
json_out=$("$BIN" doctor --json 2>/dev/null)
doctor_rc=$?
set -e

if [[ "$doctor_rc" -eq 0 ]]; then
  echo "FAIL: doctor should fail when a tracked plaintext env file is staged"
  exit 1
fi

echo "$json_out" | grep -q 'tracked_plaintext' || {
  echo "FAIL: doctor JSON should include a tracked_plaintext finding"
  exit 1
}
echo "PASS: doctor fails with tracked_plaintext finding"

echo ""
echo "=== All failure tests passed ==="
