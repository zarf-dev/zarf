#!/usr/bin/env bash

set -euo pipefail

MAIN_BRANCH="main"
TARGET_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "target branch is $TARGET_BRANCH"

mkdir -p build

git checkout $MAIN_BRANCH
go run main.go tools sbom scan . -o json --exclude './site' --exclude './examples' > build/main-syft.json

git checkout $TARGET_BRANCH
cat build/main-syft.json | grype -o template -t hack/compare.tmpl > build/main.json
go run main.go tools sbom scan . -o json --exclude './site' --exclude './examples' | grype -o template -t hack/compare.tmpl > build/target.json


result=$(jq --slurp '.[0] - .[1]' build/target.json build/main.json | jq '[.[] | select(.severity != "Low" and .severity != "Medium")]')

echo "CVEs on $MAIN_BRANCH are $(cat build/main.json | jq )"
echo "CVEs on $TARGET_BRANCH are $(cat build/target.json | jq)"

if [[ "$result" == "[]" ]]; then
  echo "no new vulnerabilities on $TARGET_BRANCH"
  exit 0
else
  echo "new CVEs have been added with IDs $result"
  exit 1
fi
