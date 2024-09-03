#!/usr/bin/env bash

set -euo pipefail

if [ -z "$(git status -s go.mod go.sum)" ]; then
    echo "Success!"
    exit 0
else
    git diff go.mod go.sum
    exit 1
fi
