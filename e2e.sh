#!/bin/bash
set -e

NOCOLOR='\033[0m'
RED='\033[0;31m'
GREEN='\033[0;32m'
ORANGE='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
YELLOW='\033[1;33m'


_run() {
    >&2 echo
    >&2 echo
    >&2 echo -e "🟢 ${GREEN}TEST RUN: ${YELLOW} $1 ${NOCOLOR}"
    # @todo: update for gitlab sec env variable injection
    ssh ec2-user@pipeline.zarf.dev "$1"
}

_curl() {
    >&2 echo
    >&2 echo
    >&2 echo -e "🟢 ${GREEN}TEST CURL: ${YELLOW} $1 ${NOCOLOR}"
    curl -sfSL --cacert zarf-ca.crt --retry 15 --retry-connrefused --retry-delay 10 "$1"
}

beforeAll() {
    # Clean the working directory
    _run "rm -fr \*"

    # Download the job artifacts
    _run "curl -fL https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/jobs/${PACKAGE_JOB_ID}/artifacts/download -o artifact.zip && unzip -jo artifact.zip"
    
    # List the downloaded files
    _run "ls -lah"

    # Sanity check the binary runs
    _run "zarf"

    # Erase any prior cluster
    _run "sudo zarf destroy --confirm"

    # Launch the utility cluster with logging and management
    _run "sudo zarf init --confirm --host=pipeline.zarf.dev --features=management,logging,utility-cluster"
}

loadZarfCA() {
    # Get the ca file for curl to trust 
    _run "sudo cat zarf-pki/zarf-ca.crt" > zarf-ca.crt
}

testAPIEndpoints() {
    # Update the CA first
    loadZarfCA

    # Test the docker registry
    _curl "https://pipeline.zarf.dev/v2/"

    # Test gitea is up and can be logged into
    _curl "https://zarf-git-user:${ZARF_PWD}@pipeline.zarf.dev/api/v1/user"

    # Test grafana is up and can be logged into
    _curl "https://zarf-admin:${ZARF_PWD}@pipeline.zarf.dev/monitor/api/org"
}

beforeAll

# Get the admin credentials 
ZARF_PWD=$(_run "sudo zarf tools get-admin-password")

# Test that k9s is happy
_run "sudo /usr/local/bin/k9s info"

# Test utility cluster and monitoring components are wup
testAPIEndpoints

#Test Zarf PKI Regenerate
_run "sudo zarf pki regenerate --host=pipeline.zarf.dev"

# Little janky, but rolling certs in traefik takes a bit to load
echo -e "${ORANGE}Sleeping for 30 seconds to wait for traefik TLS rollover${NOCOLOR}"
sleep 30

# Re-validate API endpoints with new PKI chain
testAPIEndpoints