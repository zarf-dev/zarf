## Zarf Game Mode Example

### NOTE: This a a unique implementation of Zarf, do not use the main README in the root of this repo.  You only neeed to use the instructions in this README

This example demonstrates using Zarf to kill time (and evil).  In this mode there is no utility cluster and Zarf is simple a standard means of wrapping airgap concerns for K3s.  Game mode is identical to [Appliance Mode](../appliance/README.md), but more fun. 

### Steps to use:
1. Clone this repo or copy the contents of this folder to your local machine
2. Download `zarf` and `zarf-appliance-init.tar.zst` to the same folder from the [releases page](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases).  _Note: if you are creating the package on Mac, you will also need the Mac CLI binary_
3. In that folder, run `./zarf package create` to generate the `zarf-update.tar.zst` file
4. Move the three `zarf` files to your test environment. 
5. In the test environment, run `./zarf init --appliance-mode --confirm --host=localhost`, replace `localhost` with whatever your load balancer or public access IP or DNS entry is
6. Once step 5 is complete, run `./zarf package deploy` to add the remaining cluster components.  You can rerun this step along wit 3 and 4 to make updates to the cluster

### Credits:
 - https://www.reddit.com/r/programming/comments/nap4pt/dos_gaming_in_docker/
 - https://earthly.dev/blog/dos-gaming-in-docker/
