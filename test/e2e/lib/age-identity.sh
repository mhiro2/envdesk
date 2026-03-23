#!/usr/bin/env bash
# Source from test/e2e/*.sh after set -euo pipefail. Requires cwd to be the temp repo root.

# Writes age.txt, sets RECIPIENT from age-keygen -y (avoids parsing comment lines, which differ by distro).
e2e_setup_age_identity() {
  age-keygen -o age.txt >/dev/null 2>&1
  RECIPIENT="$(age-keygen -y age.txt | tr -d '\r\n')"
  if [[ -z "${RECIPIENT}" ]]; then
    echo "FAIL: could not derive age recipient" >&2
    exit 1
  fi
  export SOPS_AGE_KEY_FILE="$(pwd)/age.txt"
}
