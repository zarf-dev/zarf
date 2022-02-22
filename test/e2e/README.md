# Zarf End-To-End Tests

This directory holds all of our e2e tests that we use to verify Zarf functionality in an environment that replicates a live setting. The tests in this directory are automatically run whenever a PR is opened against the repo and the passing of all of the tests is a pre-condition to having a PR merged into the baseline. The e2e tests stand up a KinD cluster for Zarf to use during the testing.


## Running Tests Locally

### Dependencies
Running the tests locally have the same prerequisites as running and building Zarf:
 1. GoLang >= `1.16.x`
 2. Make

### Local K8s Cluster
If you have a cluster already running on your local machine, great! As long as your kubeconfig is accessible at `~/.kube/.config` or is exposed in your `$KUBECONFIG` environment variable, the e2e tests will be able to use your cluster to run your tests. If you do not have a local cluster running, still great! The e2e tests use the `sigs.k8s.io/kind` library to stand up a local KinD cluster to test against.
> NOTE: Your existing local cluster needs to have the cluster name `test-cluster` for the e2e tests to use it

If your cluster existed before the e2e test ran, your cluster will still be up after the tests are completed. Just note that since the tests are actively using your cluster it might be in a very different state.
If you did not have a local cluster running before the e2e test but you want to keep it up afterwards to do some debugging, you can set the `SKIP_TEARDOWN` environment variable and the e2e tests will leave the create cluster up after all testing is completed.

### Actually Running The Test
Recommend running the tests by going to the directory of the Zarf repo and running `make test-new-e2e` this will guarantee all the necessary packages are built and in the right place for the test to find. If you already built everything you can run the tests by staying in this directory and using the command `go test ./... -v`

## Adding More Tests
> NOTE: Since all of the tests use the same K8s cluster, do not write new tests to be executed in parallel and remember to cleanup the cluster after each test by executing `zarf destroy --confirm --remove-components`. An example can be found [TODO PLACE LINK]

In the future, our goals is to be able to run all of the tests while using an exhaustive combination of different k8s distros and base operating systems. 


