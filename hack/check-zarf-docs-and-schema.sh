#!/usr/bin/env sh

if [ -z "$(git status -s ./site/src/content/docs/commands/ ./zarf.schema.json)" ]; then
    echo "Success!"
    exit 0
else
    git diff ./site/src/content/docs/commands/ ./zarf.schema.json
    exit 1
fi
