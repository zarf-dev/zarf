# Code Testing

Currently, we test Zarf through a series of end-to-end tests which can be found in the [e2e directory](https://github.com/defenseunicorns/zarf/tree/master/src/test/e2e) of the project. This directory holds all of our e2e tests that we use to verify Zarf functionality in an environment that replicates a live setting. The tests in this directory are automatically run against several K8s distros whenever a PR is opened or updated.

## Running Tests Locally

The tests in this directory are also able to be run locally!

### Dependencies

Running the tests locally have the same prerequisites as running and building Zarf:

1.  GoLang >= `1.18.x`
2.  Make
3.  Any clean K8s cluster (local or remote) or Linux with sudo if you want to do the Zarf-installed K3s cluster

### Actually Running The Test

Here are a few different ways to run the tests, based on your specific situation:

```shell
# Note: You can prepend CI=true to these commands to force the --no-progress flag like CI does

# The default way, from the root directory of the repo. Will run all of the tests against your chosen k8s distro. Will automatically build any binary dependencies that don't already exist.
make test-e2e ARCH="[amd64|arm64]"

# To test against a Zarf-created cluster (on Linux with sudo)
APPLIANCE_MODE=true make test-e2e ARCH="[amd64|arm64]"

# If you already have everything build, you can run this inside this folder. This lets you customize the test run.
go test ./... -v

# Let's say you only want to run one test. You would run:
test ./... -v -run TestFooBarBaz
```

:::note
The zarf binary and built packages need to live in the ./build directory but if you're trying to run the tests locally with 'go test ./...' then the zarf-init package will need to be in this directory.
:::

## Adding More Tests

There are a few requirements for all of our tests, that will need to be followed when new tests are added.

1. Tests may not run in parallel, since they use the same kubernetes cluster to run them.
2. Each test should begin with the entries below for standardization and test setup/teardown:

```go
    t.Log("E2E: Enter useful description here")
	e2e.setup(t)
	defer e2e.teardown(t)
```

## Test Naming Conventions

The tests are run sequentially and the naming convention is set intentinonally:
- 00-19 tests run prior to `zarf init` (cluster not initialized)
- 20 is reserved for `zarf init`
- 21 is reserved for logging tests so they can be removed first (they take the most resources in the cluster)
- 22 is reserved for tests required the git-server, which is removed at the end of the test
- 23-99 are for the remaining tests that only require a basic zarf cluster without logging for the git-server