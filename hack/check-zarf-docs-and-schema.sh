#!/usr/bin/env sh

if [ -z $(git status -s docs/ zarf.schema.json src/ui/lib/api-types.ts) ]; then
    echo "Success!"
    exit 0
else
    git status docs/ zarf.schema.json src/ui/lib/api-types.ts
    exit 1
fi
