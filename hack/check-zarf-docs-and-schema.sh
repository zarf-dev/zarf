#!/usr/bin/env bash

set -euo pipefail

PATHS=(
    "./zarf.schema.json"
    "./schema/zarf_package_v1alpha1.schema.json"
    "./schema/zarf_package_v1beta1.schema.json"
    "./site/src/content/docs/commands/"
)

if [ -z "$(git status -s "${PATHS[@]}")" ]; then
    echo "Success!"
    exit 0
else
    git diff "${PATHS[@]}"
    exit 1
fi
