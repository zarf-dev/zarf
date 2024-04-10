#!/usr/bin/env bash

# Directory containing the Go files
DIRECTORY="$1"

# Array of paths to exclude from the check
EXCLUDE_PATHS=(
  "src/cmd/tools/helm/repo_update.go"
  "src/cmd/tools/helm/repo_remove.go"
  "src/cmd/tools/helm/load_plugins.go"
  "src/cmd/tools/helm/repo_list.go"
  "src/cmd/tools/helm/flags.go"
  "src/cmd/tools/helm/repo_add.go"
  "src/cmd/tools/helm/dependency.go"
  "src/cmd/tools/helm/repo_index.go"
  "src/cmd/tools/helm/repo.go"
  "src/cmd/tools/helm/dependency_build.go"
  "src/cmd/tools/helm/dependency_update.go"
  "src/cmd/tools/helm/root.go"
)

BLACK='\033[0;30m'
RED='\033[0;31m'
RESET='\033[0m'

# Function to check if a path is in the EXCLUDE_PATHS array
is_excluded() {
  local path="$1"
  for exclude in "${EXCLUDE_PATHS[@]}"; do
    if [[ "$path" == "$exclude"* ]]; then
      return 0 # 0 means true/success in shell script
    fi
  done
  return 1 # 1 means false/failure in shell script
}

# Flag to track if any file meets the condition
found=0

# Use process substitution to avoid subshell issue with the 'found' variable
while IFS= read -r file; do
  if is_excluded "$file"; then
    echo -e "$BLACK$file$RESET"
    continue
  fi

  # Use `head` to grab the first two lines and compare them directly
  firstLine=$(head -n 1 "$file")
  secondLine=$(head -n 2 "$file" | tail -n 1)

  # Check if the lines do not match the specified strings
  if [[ "$firstLine" != "// SPDX-License-Identifier: Apache-2.0" || "$secondLine" != "// SPDX-FileCopyrightText: 2021-Present The Zarf Authors" ]]; then
    echo -e "$RED$file$RESET"
    found=1
  fi
done < <(find "$DIRECTORY" -type f -name "*.go")

# If any file met the condition, exit with status 1
if [ "$found" -eq 1 ]; then
  exit 1
fi
