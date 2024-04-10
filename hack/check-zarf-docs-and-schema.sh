#!/usr/bin/env bash

set -euo pipefail

if [ -z "$(git status -s docs/ zarf.schema.json)" ]; then
    echo "Success!"
    exit 0
else
    git diff docs/ zarf.schema.json
    exit 1
fi
