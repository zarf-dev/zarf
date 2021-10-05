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

_sleep() {
    echo -e "${ORANGE}Sleeping for $1 seconds${NOCOLOR}"
    sleep $1
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

    # Launch the gitops service with logging and management
    _run "sudo zarf init --confirm --host=pipeline.zarf.dev --components=management,logging,gitops-service"

    _sleep 30
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

testPrepareCommands() {
    # Validate working SHASUM computation
    EXPECTED_SHASUM="61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109"
    _run "echo 'random test data 🦄' > shasum-test-file"
    ZARF_SHASUM=$(_run "zarf prepare sha256sum shasum-test-file")
    if [ $EXPECTED_SHASUM != $ZARF_SHASUM ]; then
        echo -e "${RED}zarf prepare sha256sum failed for local file${NOCOLOR}"
        exit 1
    fi    
    # Validate working SHASUM for remote asset
    EXPECTED_SHASUM="c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617"
    ZARF_SHASUM=$(_run "zarf prepare sha256sum https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt")
    if [ $EXPECTED_SHASUM != $ZARF_SHASUM ]; then
        echo -e "${RED}zarf prepare sha256sum failed for remote file${NOCOLOR}"
        exit 1
    fi
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
    _run "sudo zarf package deploy $PACKAGE --confirm"
    # Test to confirm the root file was placed
    _run "sudo /usr/local/bin/kubectl -n demo exec data-injection -- ls /test | grep this-is-an-example"
    # Test to confirm the subdirectory file was placed
    _run "sudo /usr/local/bin/kubectl -n demo exec data-injection -- ls /test/subdirectory-test | grep this-is-an-example"
}

testGitBasedHelmChart() {
    # Create the package
    pushd examples/single-big-bang-package
    PACKAGE="zarf-package-big-bang-single-package-demo.tar.zst"
    ../../build/zarf package create --confirm
    _send $PACKAGE
    popd
    # Deploy the package
    _run "sudo zarf package deploy $PACKAGE --confirm"
    _sleep 60
    # Test to confirm the Twistlock Console was deployed
    _curl "https://pipeline.zarf.dev/api/v1/settings/initialized?project=Central+Console"
}

beforeAll

testPrepareCommands

# Get the admin credentials 
ZARF_PWD=$(_run "sudo zarf tools get-admin-password")

# Test that k9s is happy
_run "sudo /usr/local/bin/k9s info"

# Test gitops service and monitoring components are up
testAPIEndpoints

# Remove the top-level ingress, hack until we parallize these tests
_run "sudo /usr/local/bin/kubectl -n git delete ingress git-ingress"

# Test Zarf PKI Regenerate, final testing doesn't occur until after loadZarfCA and testGitBasedHelmChart
_run "sudo zarf pki regenerate --host=pipeline.zarf.dev"

# Update the CA first
loadZarfCA

# Run the data injection test
testDataInjection

# Run the helm chart tests for git-based charts (Big Bang)
testGitBasedHelmChart

# Perform final cleanup
afterAll