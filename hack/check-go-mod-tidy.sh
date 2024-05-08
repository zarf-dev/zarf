#!/usr/bin/env bash

set -euo pipefail

go mod tidy
if ! git diff --quiet go.mod go.sum; then
  echo "ERROR: Changes detected after running 'go mod tidy'. Please run 'go mod tidy' and commit the changes."
  exit 1
fi
