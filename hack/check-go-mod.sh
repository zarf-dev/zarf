#!/usr/bin/env bash

set -euo pipefail

git status

if [ -z "$(git status -s go.mod go.sum)" ]; then
    echo "Success!"
    exit 0
else
    echo "failure, please run go mod tidy and commit"
    exit 1
fi
