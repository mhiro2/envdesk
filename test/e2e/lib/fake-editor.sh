#!/usr/bin/env bash
# Fake editor that appends a new key to env files
set -euo pipefail

FILE="$1"

# Append a new key to the file
echo "" >> "$FILE"
echo "NEW_KEY=fake_editor_added" >> "$FILE"
