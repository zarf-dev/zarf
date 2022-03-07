# Zarf End-To-End Tests

This directory holds all of our e2e tests that we use to verify Zarf functionality in an environment that replicates a live setting. The tests in this directory are automatically run against all the default K8s distros whenever a PR is opened against the repo. 


# Running Tests Locally
The tests in this directory are also able to be run locally!

## Dependencies
Running the tests locally have the same prerequisites as running and building Zarf:
 1. GoLang >= `1.16.x`
 2. Make
 3. Docker 


### Existing K8s Cluster
If you have a cluster already running, great! As long as your kubeconfig is accessible at `~/.kube/.config` or is exposed in your `$KUBECONFIG` environment variable, the e2e tests will be able to use your cluster to run your tests. This means that you can even test remote distros like EKS, AKS, and GKE. When the tests are completed, your cluster will still be accessible but note that since the tests are actively using your cluster it might be in a very different state.


### No Existing K8s Cluster
If you do not have a local cluster running, no worries! The e2e tests use the `sigs.k8s.io/kind` and `github.com/rancher/k3d/v5` libraries to stand up local clusters to test against.

If you want to specify which distros to run the tests against you can set the `TESTDISTRO` environment variable with a comma separated list of K8s distros to use for the testing, each distro is run iteratively. The list of potential distros lists in the `distroTests` struct in `main_test.go`. If nothing is specified, it all 'default' distros are run.

> NOTE: If running against the k3s distro you have to be 'root' to successfully create the cluster.

If you did not have a local cluster running before the e2e test but you want to keep it up afterwards to do some debugging, you can set the `SKIP_TEARDOWN` environment variable and the e2e tests will leave the create cluster up after all testing is completed.

### Actually Running The Test
We recommend running the tests by going to the main directory of the Zarf repo and running `make test-e2e` this will guarantee all the necessary packages are built and in the right place for the test to find. If you already built everything you can run the tests by staying in this directory and using the command `go test ./... -v`

> NOTE: The zarf binary and built packages need to live in the ./build directory but if you're trying to run the tests locally with 'go test ./...' then the zarf-init package will need to be in this directory.

## Adding More Tests
> NOTE: Since all of the tests use the same K8s cluster, do not write new tests to be executed in parallel and remember to cleanup the cluster after each test by executing `e2e.cleanupAfterTest(t)`. This runs a `zarf destroy --confirm --remove-components` so that the cluster is in a good enough state to run the next test against.


Coming Soon: In the future, our goals is to be able to run all of the tests while using an exhaustive combination of different k8s distros and base operating systems. 


