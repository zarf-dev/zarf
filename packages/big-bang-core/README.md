# Big Bang Core

This package deploys [Big Bang Core](https://repo1.dso.mil/platform-one/big-bang/bigbang) using Zarf.

![pods](./images/pods.png)

![helmreleases](./images/helmreleases.png)

## Known Issues

- Currently this package does the equivalent of `kustomize build | kubectl apply -f -`, which means Flux will be used to deploy everything, but it won't be watching a Git repository for changes. Upcoming work is planned to update the package so that you will be able to open up a Git repo in the private Gitea server inside the cluster, commit and push a change, and see that change get reflected in the deployment.

> NOTE:
> Big Bang requires an AMD64 system to deploy as Iron Bank does not yet support ARM.  You will need to deploy to a cluster that is running AMD64.  Specifically, M1 Apple computers are not supported locally and you will need to provision a remote cluster to work with Big Bang currently.

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

### Initialize Zarf

```shell
# Initialize Zarf
./zarf init --confirm --components k3s,git-server

# (Optional) Inspect the results
./zarf tools k9s
```

### Deploy Big Bang

```shell
# Deploy Big Bang
./zarf package deploy --confirm zarf-package-big-bang-core-demo-amd64.tar.zst

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

## Services

| URL                                                   | Username  | Password                                                                                                                                                                                   | Notes                                                               |
| ----------------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- |
| [AlertManager](https://alertmanager.bigbang.dev:8443) | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Grafana](https://grafana.bigbang.dev:8443)           | `admin`   | `prom-operator`                                                                                                                                                                            |                                                                     |
| [Kiali](https://kiali.bigbang.dev:8443)               | n/a       | `kubectl get secret -n kiali -o=json \| jq -r '.items[] \| select(.metadata.annotations."kubernetes.io/service-account.name"=="kiali-service-account") \| .data.token' \| base64 -d; echo` |                                                                     |
| [Kibana](https://kibana.bigbang.dev:8443)             | `elastic` | `kubectl get secret -n logging logging-ek-es-elastic-user -o=jsonpath='{.data.elastic}' \| base64 -d; echo`                                                                                |                                                                     |
| [Prometheus](https://prometheus.bigbang.dev:8443)     | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Jaeger](https://tracing.bigbang.dev:8443)            | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Twistlock](https://twistlock.bigbang.dev:8443)       | n/a       | n/a                                                                                                                                                                                        | Twistlock has you create an admin account the first time you log in |

## Troubleshooting

### My computer crashed!
Close all those hundreds of chrome tabs, shut down all non-essential programs, and try again. Big Bang is a HOG. If you have less than 32GB of RAM you're in for a rough time.
