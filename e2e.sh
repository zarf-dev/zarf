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

_send() {
    >&2 echo
    >&2 echo
    >&2 echo -e "💿 ${ORANGE}COPY PACKAGE: ${YELLOW} $1 ${NOCOLOR}"
    scp "$1" ec2-user@pipeline.zarf.dev:/opt/zarf/$1
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

    # Add a delay here since we don't have a reliable way to wait for everything and curl can throw an error on a 404
    sleep 15
}

afterAll() {
    # Erase any prior cluster
    _run "sudo zarf destroy --confirm"

    # Clean the working directory
    _run "rm -fr \*"
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

testDataInjection() {
    pushd examples/data-injection
    PACKAGE="zarf-package-data-injection-demo.tar"
    ../../build/zarf package create --confirm
    _send $PACKAGE
    _run "sudo zarf package deploy $PACKAGE --confirm"
    # Test to confirm the root file was placed
    _run "sudo /usr/local/bin/kubectl -n demo exec data-injection -- ls /test | grep this-is-an-example"
    # Test to confirm the subdirectory file was placed
    _run "sudo /usr/local/bin/kubectl -n demo exec data-injection -- ls /test/subdirectory-test | grep this-is-an-example"
    popd
}

testGitBasedHelmChart() {
    pushd examples/single-big-bang-package
    PACKAGE="zarf-package-big-bang-single-package-demo.tar.zst"
    _send $PACKAGE
    _run "sudo zarf package deploy $PACKAGE --confirm"
    # Test to confirm the Twistlock Console was deployed
    _curl "https://pipeline.zarf.dev/api/v1/settings/initialized?project=Central+Console"
    popd
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

# Run the data injection test
testDataInjection

# Run the helm chart tests for git-based charts (Big Bang)
testGitBasedHelmChart

# Perform final cleanup
afterAll