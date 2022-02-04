# Example: Big Bang Core

This example shows a deployment of [Big Bang Core](https://repo1.dso.mil/platform-one/big-bang/bigbang) using Zarf.

![pods](img/pods.png)

![helmreleases](img/helmreleases.png)

## Known Issues

- Due to issues with Elasticsearch this example doesn't work yet in some distros. It does work in the Vagrant VM detailed below. Upcoming work to update to the latest version of Big Bang and swap the EFK stack out for the PLG stack (Promtail, Loki, Grafana) should resolve this issue

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make`
1. Install `sha256sum` (on Mac it's `brew install coreutils`)

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

### Build the deploy package

```shell
# Create the deploy package and move it to the 'examples/sync' folder
make package-example-big-bang
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

### Deploy Big Bang

```shell
# Deploy Big Bang
./zarf package deploy --confirm zarf-package-big-bang-core-demo.tar.zst --components kubescape

# (Optional) Inspect the results
./zarf tools k9s
```

### Clean Up

```shell
# Inside the VM
exit

# On the host
make vm-destroy
```

## Kubescape scan

This example adds the `kubescape` binary, which can scan clusters for compliance with the NSA/CISA Kubernetes Hardening Guide

```shell
kubescape scan framework nsa --use-from=/usr/local/bin/kubescape-framework-nsa.json --exceptions=/usr/local/bin/kubescape-exceptions.json
```

## Services

| URL                                                   | Username  | Password                                                                                                                                                                                   | Notes                                                               |
| ----------------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- |
| [AlertManager](https://alertmanager.bigbang.dev:9443) | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Grafana](https://grafana.bigbang.dev:9443)           | `admin`   | `prom-operator`                                                                                                                                                                            |                                                                     |
| [Kiali](https://kiali.bigbang.dev:9443)               | n/a       | `kubectl get secret -n kiali -o=json \| jq -r '.items[] \| select(.metadata.annotations."kubernetes.io/service-account.name"=="kiali-service-account") \| .data.token' \| base64 -d; echo` |                                                                     |
| [Kibana](https://kibana.bigbang.dev:9443)             | `elastic` | `kubectl get secret -n logging logging-ek-es-elastic-user -o=jsonpath='{.data.elastic}' \| base64 -d; echo`                                                                                |                                                                     |
| [Prometheus](https://prometheus.bigbang.dev:9443)     | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Jaeger](https://tracing.bigbang.dev:9443)            | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Twistlock](https://twistlock.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | Twistlock has you create an admin account the first time you log in |
