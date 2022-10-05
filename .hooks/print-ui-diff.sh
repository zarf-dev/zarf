#!/usr/bin/env sh

# Get the diff for UI related files
git diff src/ui
git diff package.json
git diff package-lock.json
git diff .npmrc
git diff .eslint*
git diff ts*
git diff prettier*
git diff svelte*
git diff vite*
git diff playwright*

# Get the current commit, branch and other information
git show --oneline -s
