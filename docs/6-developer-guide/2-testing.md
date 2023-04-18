# Code Testing



Currently, we primarily test Zarf through a series of end-to-end tests located in the [e2e directory](https://github.com/defenseunicorns/zarf/tree/main/src/test/e2e) of the project. This directory houses all of the e2e tests that we use to verify Zarf's functionality in an environment that replicates a live setting. The tests in this directory undergo automatic execution against several K8s distros whenever a pull request is created or updated. Through this testing, we ensure that Zarf performs consistently across a range of K8s environments, ensuring its reliability for users.

In addition, Zarf undergoes a series of unit tests for specific functions where edge cases prove difficult to cover through end-to-end testing alone. You can locate these tests in the [src/pkg directory](https://github.com/defenseunicorns/zarf/tree/main/src/pkg), where they are identified by `*_test.go` files.

## Dependencies

To run the end-to-end tests locally, you must meet the same prerequisites as those required for building and running Zarf, which include:

1. GoLang >= `1.19.x`.
2. Make.
3. Any clean K8s cluster (local or remote) or Linux with sudo if you want to use the Zarf-installed K3s cluster.
4. NodeJS >= `18.x.x`.

### CLI End-to-End Tests

There are several ways to run tests depending on your specific situation, such as:

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
The Zarf binary and built packages are required to be stored in the ./build directory. However, if you intend to run tests locally using 'go test ./...', the zarf-init package must also be present in this directory.
:::

### Adding New CLI End-to-End Tests

When adding new tests, there are several requirements that must be followed, including:

1. Tests cannont be run in parallel as they utilize the same K8s cluster.
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

- 00-19 tests run prior to `zarf init` (cluster not initialized).

:::note
Tests 20+ should call `e2e.setupWithCluster(t)` instead of `e2e.setup(t)`.

Due to resource constraints in public GitHub runners, K8s tests are only performed on Linux.
:::

- 20 is reserved for `zarf init`.
- 21 is reserved for logging tests so they can be removed first (they take the most resources in the cluster).
- 22 is reserved for tests required the git-server, which is removed at the end of the test.
- 23-98 are for the remaining tests that only require a basic Zarf cluster without logging for the git-server.
- 99 is reserved for the `zarf destroy` and [YOLO Mode](../../examples/yolo/README.md) test.

## CLI Unit Tests

### Running CLI Unit Tests

There are several ways to run tests depending on your specific situation, such as:

``` bash
# The default way, from the root directory of the repo. Will run all of the unit tests that are currently defined.
make test-unit

# If you already have everything built, you can run this inside this folder. This lets you customize the test run.
go test ./src/pkg/... -v

# Let's say you only want to run one test. You would run:
go test ./src/pkg/... -v -run TestFooBarBaz
```

### Adding New CLI Unit Tests

When adding new unit tests, please ensure that the following requirements are met:

1. The test must focus on a true unit, such as a single function or file.
2. The code being tested must have a clearly defined interface, such as a public specification.
3. The code being tested should be located within the `src/pkg`.

If all these requirements are met, then a unit test would be appropriate. If not, please consider writing an end-to-end test instead or modify your approach to meet these requirements.

To generate a unit test, search for or include a file that ends with `_test.go` to the package for the file that requires testing, such as `auth.go` -> `auth_test.go`. Import the testing library and create test functions as necessary. In case you need to mock something out, determine the most suitable approach and if the mock can be utilized in multiple tests, consider placing it in  `./src/test/mocks/`. This will help enhance the efficiency and organization of your unit tests.

## UI End-to-End Tests

The end-to-end tests for the UI are executed through [Playwright](https://playwright.dev/), which is a NodeJS library designed for running end-to-end tests against a browser. These tests are run against the Zarf UI and can be located in the `./src/test/ui` directory. By utilizing Playwright, developers can verify the functionality of the UI in a realistic and reliable manner, ensuring that it meets the intended requirements and user experience. The location of the UI tests in the directory also allows for easy access and maintenance of the tests.

### Running UI End-to-End Tests

There are several ways to run tests depending on your specific situation, such as:

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
