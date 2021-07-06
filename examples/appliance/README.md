## Zarf Appliance Mode Example

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no utility cluster and Zarf is simple a standard means of wrapping airgap concerns for K3s.  Appliance mode is also unique because you do not use anyting from the repo [releases](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases) except the CLI.  This mode requires creating your own `zarf-initiazlize.tar.zst` to deploy the assets.  Though there are more complex patterns that could use the update process as well, for this example we only ever create the initial deployment, therefore updates are done by re-creating the environment. 

### Steps to use:
1. Download the Zarf linux CLI from the [releases page](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases).  _Note: if you are creating the package on Mac, you will also need the Mac CLI binary_
2. Within this folder, run `./zarf package create`
3. Take the `zarf` CLI binary and the created `zarf-initialize.tar.zst` and move them to your test environment.  _Note: you can use the `make test OS=ubuntu` target in the root of this repo to test with vagrant if you place these two files in the `build` directory_
4. In the test environment, run `./zarf initialize --confirm --host=localhost`, replace `localhost` with whatever your load balancer or public access IP or DNS entry is
5. Profit