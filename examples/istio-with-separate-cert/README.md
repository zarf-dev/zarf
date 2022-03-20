# Example: Big Bang's Istio with a separately loaded cert

This example deploys Big Bang's Istio, but without an ingress cert. It is applicable in use cases where you want to have a freely distributable zarf package, but your ingress cert is private and can't be distributed in the same way that you want the Zarf package to be.

## Known Issues

The same known issues that are documented in [the Big Bang example](../big-bang/README.md#known-issues) apply here as well, except for the Elasticsearch stuff since we have EFK turned off.

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
1. Install `make`
1. Install `sha256sum` (on Mac it's `brew install coreutils`)
1. [Logged into registry1.dso.mil](https://github.com/defenseunicorns/zarf/blob/master/docs/ironbank.md#2-configure-zarf-the-use-em)

## Instructions

### Pull down the code and binaries

```shell
# clone the binaries
git clone https://github.com/defenseunicorns/zarf.git

# change to the examples folder
cd zarf/examples

# Download the latest release of Zarf and the Init Package to the 'examples/sync' folder
make fetch-release
```

> NOTE:
>
> If you have any issues with `make fetch-release` you can try `make build-release` instead. It will build the files instead of downloading them. You'll need Golang installed.

### Build the deploy package

```shell
# Create the deploy package and move it to the 'examples/sync' folder. It will also create a kubernets manifest with the '*.bigbang.dev' cert that you can use later in the example.
make package-example-istio-with-separate-cert
```

### Start the Vagrant VM

```shell
# Start the VM. You'll be dropped into a shell in the VM as the Root user
make vm-init
```

> NOTE:
>
> All subsequent commands should be happening INSIDE the Vagrant VM

### Initialize Zarf

```shell
# Initialize Zarf
./zarf init --confirm --components k3s,gitops-service

# (Optional) Inspect the results
./zarf tools k9s
```

### Deploy the package

```shell
# Deploy Big Bang
./zarf package deploy --confirm zarf-package-example-istio-with-separate-cert-amd64.tar.zst

# (Optional) Inspect the results
./zarf tools k9s
```

### Delete buggy EnvoyFilter

Wait until Istio is running, then delete this EnvoyFilter. It doesn't work correctly due to a bug.

```shell
# Delete this EnvoyFilter, it is bugged. Will be fixed when we update to a later version of Big Bang
kubectl delete -n istio-system envoyfilter/misdirected-request
```

### Deploy the TLS cert

First, go to [https://kiali.bigbang.dev:8443](https://kiali.bigbang.dev:8443) just to see that it doesn't work, because Istio doesn't have a TLS cert to use.

```shell
# Create the cert
kubectl create secret tls public-cert-actual -n istio-system --cert bigbangdev.cert --key bigbangdev.key
```

Then, try going back to [https://kiali.bigbang.dev:8443](https://kiali.bigbang.dev:8443). It should work this time.

### Clean Up

```shell
# Inside the VM
exit

# On the host
make vm-destroy
```

## Notes for Maintainers

- The `*.bigbang.dev` cert expires every 90 days. To regenerate the latest one run `cd examples && make generate-bigbang-dev-cert`. Requires `curl` and [`yq`](https://github.com/mikefarah/yq/).
