# Example: Big Bang Core All-In-One

This example deploys Big Bang Core to a Utility Cluster. This is not normally the method that will be used in production but for a demo it works great.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 8443

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make` and `kustomize`

## Instructions

1. From within the examples directory, Run: `make all`
5. Run: `sudo su` - Change user to root
6. Run: `cd zarf-examples` - Change to the directory where the examples folder is mounted
7. Run: `./zarf init --confirm --features management,utility-cluster --host localhost` - Initialize Zarf, telling it to install the management feature and utility cluster and skip logging feature (since BB has logging already) and tells Zarf to use `localhost` as the domain
8. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
9. Run: `./zarf package deploy zarf-package-big-bang-core-demo.tar.zst --confirm` - Deploy Big Bang Core
10. Wait several minutes. Run `k9s` to watch progress
11. Use a browser to visit the various services, available at https://*.bigbang.dev:8443
12. When you're done, run `make vm-destroy` to bring everything down

## To-Do

1. Re-enable the NetworkPolicies - They got disabled to resolve an issue connecting to the k8s cluster API server, which is fine for a demo but unacceptable in production
