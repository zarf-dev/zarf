## Zarf Appliance Mode Example

### NOTE: This a a unique implementation of Zarf, do not use the main README in the root of this repo.  You only neeed to use the instructions in this README

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no utility cluster and Zarf is simple a standard means of wrapping airgap concerns for K3s.  Appliance mode is also unique because you do not use anyting from the repo [releases](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases) except the CLI.  This mode requires creating your own `zarf-initiazlize.tar.zst` to deploy the assets.  Though there are more complex patterns that could use the update process as well, for this example we only ever create the initial deployment, therefore updates are done by re-creating the environment. This example deploys a basic K3s cluster using Traefik 2 and configures TLS / airgap concerns to deploy [Podinfo](https://github.com/stefanprodan/podinfo).

### Steps to use:
1. Clone this repo or copy the contents of this folder to your local machine
2. Download `zarf` and `zarf-appliance-init.tar.zst` to the same folder from the [releases page](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases).  _Note: if you are creating the package on Mac, you will also need the Mac CLI binary_
3. In that folder, run `./zarf package create` to generate the `zarf-update.tar.zst` file
4. Move the three `zarf` files to your test environment. 
5. In the test environment, run `./zarf init --appliance-mode --confirm --host=localhost`, replace `localhost` with whatever your load balancer or public access IP or DNS entry is
6. Once step 5 is complete, run `./zarf package deploy` to add the remaining cluster components.  You can rerun this step along wit 3 and 4 to make updates to the cluster

### Test Locally:
You can run `make run-example KIND=appliance` from the root of this repo (if you cloned it) to build and deploy this example using Vagrant on Ubuntu.