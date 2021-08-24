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
    $1
}

_sleep() {
    echo -e "${ORANGE}Sleeping for $1 seconds${NOCOLOR}"
    sleep $1
}

beforeAll() {
    # Erase any prior cluster
    _run "zarf destroy --confirm"

    # Launch the utility cluster with logging and management
    _run "zarf init --confirm --host=pipeline.zarf.dev --features=management,logging,utility-cluster"

    _sleep 30
}

afterAll() {
    # Erase any prior cluster
    _run "zarf destroy --confirm"

    # Clean the working directory
    _run "rm -fr \*"
}

loadZarfCA() {
    # Get the ca file for curl to trust
    _run "cat zarf-pki/zarf-ca.crt" > zarf-ca.crt
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
    # Create the package
    pushd examples/data-injection
    PACKAGE="zarf-package-data-injection-demo.tar"
    ../../build/zarf package create --confirm
    _send $PACKAGE
    popd
    # Deploy the package
    _run "zarf package deploy $PACKAGE --confirm"
    # Test to confirm the root file was placed
    _run "/usr/local/bin/kubectl -n demo exec data-injection -- ls /test | grep this-is-an-example"
    # Test to confirm the subdirectory file was placed
    _run "/usr/local/bin/kubectl -n demo exec data-injection -- ls /test/subdirectory-test | grep this-is-an-example"
}

testGitBasedHelmChart() {
    # Create the package
    pushd examples/single-big-bang-package
    PACKAGE="zarf-package-big-bang-single-package-demo.tar.zst"
    ../../build/zarf package create --confirm
    _send $PACKAGE
    popd
    # Deploy the package
    _run "zarf package deploy $PACKAGE --confirm"
    _sleep 30
    # Test to confirm the Twistlock Console was deployed
    _curl "https://pipeline.zarf.dev/api/v1/settings/initialized?project=Central+Console"
}

beforeAll

## Get the admin credentials
#ZARF_PWD=$(_run "zarf tools get-admin-password")
#
## Test that k9s is happy
#_run "/usr/local/bin/k9s info"
#
## Test utility cluster and monitoring components are wup
#testAPIEndpoints
#
## Remove the top-level ingress, hack until we parallize these tests
#_run "/usr/local/bin/kubectl -n git delete ingress git-ingress"
#
## Test Zarf PKI Regenerate, final testing doesn't occur until after loadZarfCA and testGitBasedHelmChart
#_run "zarf pki regenerate --host=pipeline.zarf.dev"
#
## Update the CA first
#loadZarfCA
#
## Run the data injection test
#testDataInjection
#
## Run the helm chart tests for git-based charts (Big Bang)
#testGitBasedHelmChart
#
## Perform final cleanup
#afterAll
