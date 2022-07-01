# Zarf End-To-End Tests

This directory holds all of our e2e tests that we use to verify Zarf functionality in an environment that replicates a live setting. The tests in this directory are automatically run against several K8s distros whenever a PR is opened or updated.

## Running Tests Locally
The tests in this directory are also able to be run locally!

### Dependencies
Running the tests locally have the same prerequisites as running and building Zarf:
 1. GoLang >= `1.18.x`
 2. Make
 3. Access to a K8s cluster to test against or Linux with root priv if `APPLIANCE_MODE=true`
 4. (for the `k3s` cluster only) Linux with root privileges

### Actually Running The Test
Here are a few different ways to run the tests, based on your specific situation:

```shell
# The default way, from the root directory of the repo. Will run all of the tests against your chosen k8s distro. Will automatically build any binary dependencies that don't already exist.
APPLIANCE_MODE=true|false make test-e2e ARCH=arm64|amd64

# If you already have everything build, you can run this inside this folder. This lets you customize the test run.
go test ./... -v

# Let's say you only want to run one test. You would run:
go test ./... -v -run TestFooBarBaz
```

> NOTE: The zarf binary and built packages need to live in the ./build directory but if you're trying to run the tests locally with 'go test ./...' then the zarf-init package will need to be in this directory.

## Adding More Tests
There are a few requirements for all of our tests, that will need to be followed when new tests are added.

1. Tests may not run in parallel, since they use the same kubernetes cluster to run them.
2. The following lines must be at the start of each test:
   ```go
    t.Log("E2E: Test description")
	e2e.setup(t)
	defer e2e.teardown(t)
    ```
