# Code Testing

:::caution Hard Hat Area
This page is still being developed. More content will be added soon!
:::


Currently, we test Zarf through a series of end-to-end tests which can be found in the [e2e directory](https://github.com/defenseunicorns/zarf/tree/master/src/test/e2e) of the project. This directory holds all of our e2e tests that we use to verify Zarf functionality in an environment that replicates a live setting. The tests in this directory are automatically run against several K8s distros whenever a PR is opened or updated.

## Running Tests Locally
The tests in this directory are also able to be run locally!

### Dependencies
Running the tests locally have the same prerequisites as running and building Zarf:
 1. GoLang >= `1.18.x`
 2. Make
 3. (for the `kind` and `k3d` options only) Docker
 4. (for the `k3s` cluster only) Linux with root privileges

### Existing K8s Cluster
If you have a cluster already running, and use the env var `TESTDISTRO=provided`, the test suite will use the `KUBECONFIG` env var and the cluster that is currently configured as the active context. To make sure you are running in the right cluster, run something like `kubectl get nodes` with no optional flags and if the nodes that appear are the ones you expect, then the tests will use that cluster as well.

This means that you are able to run the tests against any remote distro you want, like EKS, AKS, GKE, RKE, etc.

### No Existing K8s Cluster
If you do not have a local cluster running, no worries! The e2e tests use the `sigs.k8s.io/kind` and `github.com/k3d-io/k3d/v5` libraries to stand up local clusters to test against. All you have to do is make sure Docker is running and set the `TESTDISTRO` env var to either `"kind"` or `"k3d"` and the test suite will automatically create the appropriate cluster before the test run, run the tests on it, then automatically destroy it to clean up.

You can also use K3s by setting `TESTDISTRO=k3s` but note that there are extra requirements of being on Linux with root privileges.

### Actually Running The Test
Here are a few different ways to run the tests, based on your specific situation:

```shell
# The default way, from the root directory of the repo. Will run all of the tests against your chosen k8s distro. Will automatically build any binary dependencies that don't already exist.
TESTDISTRO="[provided|kind|k3d|k3s]" make test-e2e

# If you already have everything build, you can run this inside this folder. This lets you customize the test run.
TESTDISTRO=YourChoiceHere go test ./... -v

# Let's say you only want to run one test. You would run:
TESTDISTRO=YourChoiceHere go test ./... -v -run TestFooBarBaz
```
:::note
The zarf binary and built packages need to live in the ./build directory but if you're trying to run the tests locally with 'go test ./...' then the zarf-init package will need to be in this directory.
:::

## Adding More Tests
There are a few requirements for all of our tests, that will need to be followed when new tests are added.

1. Tests may not run in parallel, since they use the same kubernetes cluster to run them.
2. Each test must begin with `defer e2e.cleanupAfterTest(t)` so that the cluster can be reset back to empty when finished.

## Coming Soon
1. More Linux distros tested
2. More K8s distros tested, including cloud distros like EKS
3. Make the tests that run in the CI pipeline more efficient by using more parallelization