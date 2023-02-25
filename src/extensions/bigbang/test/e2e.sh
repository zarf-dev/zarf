#!/bin/bash

# This test assumes Linux AMD64

# Get the Big Bang release versions
project="2872"
releases=$(curl -s https://repo1.dso.mil/api/v4/projects/$project/repository/tags | jq -r '[.[] | select(.name | test("^[^-]+$")) | .name][0:2] | .[]')
latest=$(echo "$releases" | awk 'NR==1')
previous=$(echo "$releases" | awk 'NR==2')

echo "Latest: $latest"
echo "Previous: $previous"

export PATH=$(pwd)/build:$PATH

# Build the CLI and init-package
make build-cli-linux-amd init-package

# Initialize the cluster
zarf init --confirm --components git-server

# Build and deploy the previous version
zarf package create src/extensions/bigbang/test/package --set=BB_VERSION=$previous --confirm
zarf package deploy zarf-package-big-bang-test-amd64-$previous.tar.zst --confirm

# Cluster info
kubectl describe nodes

# Build the latest version
zarf package create src/extensions/bigbang/test/package --set=BB_VERSION=$latest --confirm

# Cleanup to avoid any disk pressure issues on GH runners
rm -f zarf-package-big-bang-test-amd64-$previous.tar.zst 
zarf tools clear-cache

# Deploy the latest version
zarf package deploy zarf-package-big-bang-test-amd64-$latest.tar.zst --confirm

# Cluster info
kubectl describe nodes
