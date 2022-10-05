#!/usr/bin/env sh

# Get the diff for UI related files
git diff src/ui
git diff package.json
git diff package-lock.json

# Get the current commit, branch and other information
git show --oneline -s
