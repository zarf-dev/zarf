## Zarf Game Mode Example

### NOTE: This a a unique implementation of Zarf, do not use the main README in the root of this repo.  You only neeed to use the instructions in this README

This example demonstrates using Zarf to kill time (and evil).  In this mode there is no utility cluster and Zarf is simple a standard means of wrapping airgap concerns for K3s.  Game mode is identical to [Appliance Mode](../appliance/README.md), but more fun. 

### Steps to use:
1. Clone this repo or copy the contents of this folder to your local machine
2. Download the Zarf linux CLI to the same folder from the [releases page](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases).  _Note: if you are creating the package on Mac, you will also need the Mac CLI binary_
3. Within this folder, run `./zarf package create`
4. Take the `zarf` CLI binary and the created `zarf-initialize.tar.zst` and move them to your test environment.  _Note: you can use the `make test OS=ubuntu` target in the root of this repo to test with vagrant if you place these two files in the `build` directory_
5. In the test environment, run `./zarf initialize --confirm --host=localhost`, replace `localhost` with whatever your load balancer or public access IP or DNS entry is
6. Profit

### Test Locally:
You can run `make run-example KIND=game` from the root of this repo (if you cloned it) to build and deploy this example using Vagrant on Ubuntu.

### Credits:
 - https://www.reddit.com/r/programming/comments/nap4pt/dos_gaming_in_docker/
 - https://earthly.dev/blog/dos-gaming-in-docker/
