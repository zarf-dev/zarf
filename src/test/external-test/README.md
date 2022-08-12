# Test Initializing Zarf w/ An External Git Repository and A External Container Registry
> Note: In this case, the 'external' Git server and container registry are both considered 'external' servers that already existed inside the k8s cluster before `zarf init` is executed

This directory holds the tests that verify Zarf can initialize a cluster to use an already existing Git server and container registry that is external to the resources Zarf manages. The tests in this directory are currently only run when manually executed.


## Running Tests Locally

### Dependencies
Running the tests locally have the same prerequisites as running and building Zarf:
 1. GoLang >= `1.19.x`
 2. Access to a K8s cluster to test against
    - (for the internal `k3s` cluster only) Linux with root privileges
3. The Zarf binary to be on your path and aliased to `zarf`
4. The `zarf-init` package built (the `zarf.yaml` file at the root of this project)
5. The `examples/flux-test/zarf.yaml` package built

### Actually Running The Test
Here are a few different ways to run the tests, based on your specific situation:

```shell
# If you have met all the dependencies and you are currently inside this folder:
go test ./... -v
```

```shell
# If you are in the root folder of the repository and don't have the dependencies met:
make build-cli init-package
export PATH=$PATH:{path/to/zarf-repo}/build
zarf package create examples/flux-test --confirm
mv zarf-package* build/
go test ./src/test/external-git/...
```
