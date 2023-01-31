# Code Testing

Currently, we primarily test Zarf through a series of end-to-end tests which can be found in the [e2e directory](https://github.com/defenseunicorns/zarf/tree/main/src/test/e2e) of the project. This directory holds all of the e2e tests that we use to verify Zarf functionality in an environment that replicates a live setting. The tests in this directory are automatically run against several K8s distros whenever a PR is opened or updated.

For certain functions, we also test Zarf with a set of unit tests where there are edge cases that are difficult to fully flesh out with an end-to-end test alone.  These tests are located as `*_test.go` files within the [src/pkg directory](https://github.com/defenseunicorns/zarf/tree/main/src/pkg).

## Dependencies

Running the end-to-end tests locally have the same prerequisites as running and building Zarf:

1. GoLang >= `1.19.x`
2. Make
3. Any clean K8s cluster (local or remote) or Linux with sudo if you want to do the Zarf-installed K3s cluster
4. NodeJS >= `18.x.x`

### CLI End-To-End Tests

Here are a few different ways to run the tests, based on your specific situation:

``` bash
# Note: You can prepend CI=true to these commands to force the --no-progress flag like CI does

# The default way, from the root directory of the repo. Will run all of the tests against your chosen k8s distro. Will automatically build any binary dependencies that don't already exist.
make test-e2e ARCH="[amd64|arm64]"

# To test against a Zarf-created cluster (on Linux with sudo)
APPLIANCE_MODE=true make test-e2e ARCH="[amd64|arm64]"

# If you already have everything build, you can run this inside this folder. This lets you customize the test run.
go test ./src/test/... -v

# Let's say you only want to run one test. You would run:
go test ./src/test/... -v -run TestFooBarBaz
```

:::note
The zarf binary and built packages need to live in the ./build directory but if you're trying to run the tests locally with 'go test ./...' then the zarf-init package will need to be in this directory.
:::

### Adding More CLI End-to-End Tests

There are a few requirements for all of our tests, that will need to be followed when new tests are added.

1. Tests may not run in parallel, since they use the same kubernetes cluster to run them.
2. Each test should begin with the entries below for standardization and test setup/teardown:

```go
func TestFooBarBaz(t *testing.T) {
    t.Log("E2E: Enter useful description here")
    e2e.setup(t)
    defer e2e.teardown(t)

    ...
}
```

### CLI End-to-End Test Naming Conventions

The end-to-end tests are run sequentially and the naming convention is set intentionally:

- 00-19 tests run prior to `zarf init` (cluster not initialized)

:::note
Tests 20+ should call `e2e.setupWithCluster(t)` instead of `e2e.setup(t)`

Due to resource constraints in public github runners, K8s tests are only performed on Linux
:::

- 20 is reserved for `zarf init`
- 21 is reserved for logging tests so they can be removed first (they take the most resources in the cluster)
- 22 is reserved for tests required the git-server, which is removed at the end of the test
- 23-98 are for the remaining tests that only require a basic zarf cluster without logging for the git-server
- 99 is reserved for the `zarf destroy` and [YOLO Mode](../../examples/yolo/README.md) test

## CLI Unit Tests

### Running CLI Unit Tests

Here are a few different ways to run the tests, based on your specific situation:

``` bash
# The default way, from the root directory of the repo. Will run all of the unit tests that are currently defined.
make test-unit

# If you already have everything built, you can run this inside this folder. This lets you customize the test run.
go test ./src/pkg/... -v

# Let's say you only want to run one test. You would run:
go test ./src/pkg/... -v -run TestFooBarBaz
```

### Adding More CLI Unit Tests

There are a few requirements to be considered when thinking about adding new unit tests.

1. Is what I want to test a true unit (i.e. a single function or file)?
2. Does what I want to test have a clearly defined interface (i.e. a public specification)?
3. Is this code inside of the `src/pkg` folder or should it be?

If the answer to these is yes, then this would be a great place for a unit test, if not, you should likely consider writing an end-to-end test instead, or modifying your approach so that you can answer yes.

To create a unit test, look for or add a file ending in `_test.go` to the package for the file you are looking to test (e.g. `auth.go` -> `auth_test.go`).  Import the testing library and then create your test functions as needed.  If you need to mock something out consider the best way to do this, and if it is something that can be used in many tests, consider placing the mock in `./src/test/mocks/`.

## UI End-To-End Tests

The UI end-to-end tests are run using [Playwright](https://playwright.dev/).  Playwright is a NodeJS library that allows you to run end-to-end tests against a browser.  The tests are run against the Zarf UI and are located in the `./src/test/ui` directory.

### Running UI End-To-End Tests

Here are a few different ways to run the tests, based on your specific situation:

```shell
# dont forget to install dependencies
npm ci

# get help with playwright
npx playwright --help

# run tests with @pre-init tag
npm run test:pre-init

# run tests with @init tag
npm run test:init

# run tests with @post-init tag
npm run test:post-init
```
