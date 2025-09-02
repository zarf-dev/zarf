#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT_DIR=$(builtin cd "$SCRIPT_DIR"/../.. && pwd)

TEST_GO_FILE="$REPO_ROOT_DIR/src/test/e2e/06_create_sbom_test.go"
echo "::debug::TEST_GO_FILE='$TEST_GO_FILE'"

GITEA_IMAGE=$(yq -oy '.package.create.set.gitea_image' $REPO_ROOT_DIR/zarf-config.toml | sed 's!/!_!g; s!:!_!g')
echo "::debug::GITEA_IMAGE='$GITEA_IMAGE'"

SED_REPLACE=$(printf 's!ghcr.io_go-gitea_gitea_[0-9\.]+-rootless!%s!g' "$GITEA_IMAGE")
echo "::debug::SED_REPLACE='$SED_REPLACE'"

sed -i -E "$SED_REPLACE" -- $TEST_GO_FILE
