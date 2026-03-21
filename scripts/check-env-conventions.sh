#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

failed=0

while IFS= read -r -d '' path; do
  case "$path" in
    *.example|*.example.*)
      continue
      ;;
    *.env|*.env.*|.env|.env.*)
      ;;
    *)
      continue
      ;;
  esac

  if grep -Fq 'ENC[' "$path" || grep -Fq 'sops:' "$path"; then
    continue
  fi

  printf 'tracked env file appears plaintext: %s\n' "$path" >&2
  failed=1
done < <(git ls-files -z --cached)

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi
