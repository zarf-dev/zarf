# Example: Big Bang Core All-In-One

This example deploys Big Bang Core to a Utility Cluster. This is not normally the method that will be used in production but for a demo it works great.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 9443

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make` and `kustomize`

## Instructions

1. From within the examples directory, Run: `make all`, which will download the latest built binaries, build all of the example packages, and launch a basic VM to run in. Alternatively, run `make all-dev` if you want to build the binaries using the current codebase instead of downloading them.
5. Run: `sudo su` - Change user to root
6. Run: `cd zarf-examples` - Change to the directory where the examples folder is mounted
7. Run: `./zarf init --confirm --components management,utility-cluster --host localhost` - Initialize Zarf, telling it to install the management component and utility cluster and skip logging component (since BB has logging already) and tells Zarf to use `localhost` as the domain
8. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
9. Run: `./zarf package deploy zarf-package-big-bang-core-demo.tar.zst --confirm` - Deploy Big Bang Core
10. Wait several minutes. Run `k9s` to watch progress
11. Use a browser to visit the various services, available at https://*.bigbang.dev:9443
12. When you're done, run `make vm-destroy` to bring everything down

## Kubescape scan

This example adds the `kubescape` binary, which can scan clusters for compliance with the NSA/CISA Kubernetes Hardening Guide

```shell
kubescape scan framework nsa --use-from /usr/local/bin/kubescape-framework-nsa.json
```

## To-Do

1. Re-enable the NetworkPolicies - They got disabled to resolve an issue connecting to the k8s cluster API server, which is fine for a demo but unacceptable in production
