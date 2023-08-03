#!/usr/bin/env sh

# Get the diff for UI related files
git diff HEAD src/ui
git diff HEAD .eslint*
git diff HEAD ts*
git diff HEAD prettier*
git diff HEAD svelte*
git diff HEAD vite*
git diff HEAD playwright*

# Get the current commit, branch and other information
git show --oneline -s
