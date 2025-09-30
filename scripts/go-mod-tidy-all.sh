#!/usr/bin/env bash

set -euo pipefail

find . -name go.mod -print0 | while IFS= read -r -d '' modfile; do
  echo "Updating $modfile"
  DIR=$(dirname "$modfile")
  (cd "$DIR" && go mod tidy)
done