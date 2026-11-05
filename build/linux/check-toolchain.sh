#!/usr/bin/env bash
set -euo pipefail

# Called from the repository root, including inside persistent Docker containers.
if [[ ${WAILS_EXP_USE_WAKE+x} ]]; then
  echo 'Unset WAILS_EXP_USE_WAKE: this project uses the non-experimental Taskfile build.' >&2
  exit 1
fi
expected=$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
cli=$(command -v wails3)
actual=$(go version -m "$cli" | awk '$1 == "mod" && $2 == "github.com/wailsapp/wails/v3" {print $3}')
if [[ "$actual" != "$expected" ]]; then
  echo "Wails CLI is ${actual:-unknown}; go.mod requires $expected. Rebuild the Docker image and create a new container, or install github.com/wailsapp/wails/v3/cmd/wails3@$expected." >&2
  exit 1
fi
