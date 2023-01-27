# Big Bang Core

This package deploys [Big Bang Core](https://repo1.dso.mil/platform-one/big-bang/bigbang) using Zarf.

![pods](./images/pods.png)

![helmreleases](./images/helmreleases.png)

## Known Issues

- Big Bang requires an AMD64 system to deploy as Iron Bank does not yet support ARM.  You will need to deploy to a cluster that is running AMD64.  Specifically, M1 Apple computers are not supported locally and you will need to provision a remote cluster to work with Big Bang currently.

## Instructions

### Pull down the code and binaries

``` bash
# Clone the binaries
git clone https://github.com/defenseunicorns/zarf.git

# Change dir
cd zarf
```

### Get K3d components

Follow instructions on [this page](../../docs/13-walkthroughs/index.md#walk-through-prerequisites) for docker and the `k3d` cli

### Get Zarf components

Follow instructions on  https://zarf.dev/install/ to get the `zarf` cli

(Optional) Alternatively, build the zarf components from the repo
``` bash
# Build zarf components from scratch (NOTE: golang and npm must be installed)
make init-package

# Add zarf cli from build dir to path
export PATH=$(pwd)/build:$PATH
```

### Build the deploy package

``` bash
# Change dir
cd packages/big-bang-core

# Authenticate to the registry with Big Bang artifacts
set +o history
export REGISTRY1_USERNAME=<REPLACE_ME>
export REGISTRY1_PASSWORD=<REPLACE_ME>
echo $REGISTRY1_PASSWORD | zarf tools registry login registry1.dso.mil --username $REGISTRY1_USERNAME --password-stdin
set -o history

# Run zarf package command
zarf package create . --confirm
```

### Initialize Zarf

``` bash
# Start k3d cluster
k3d cluster create

# Initialize Zarf (interactively)
zarf init
# Make these choices at the prompt
# ? Do you want to download this init package? Yes
# ? Deploy this Zarf package? Yes
# ? Deploy the k3s component? No
# ? Deploy the logging component? No
# ? Deploy the git-server component? Yes

# (Optional) An alternative approach is to get the zarf init package from the zarf repo releases page or via build
# Change dir to location of the zarf-init*.tar.zst (such as the build dir) & run the zarf init command with these flags
cd ../../build
zarf init --confirm --components git-server

# (Optional) Inspect the results
zarf tools k9s
```

### Deploy Big Bang

``` bash
# Deploy Big Bang (lightweight version)
cd ../packages/big-bang-core
zarf package deploy --confirm $(ls -1 zarf-package-big-bang-core-demo-*.tar.zst) --components big-bang-core-limited-resources
# NOTE: to deploy the standard full set of components use the flag:
# '--components big-bang-core-standard'

# (Optional) Inspect the results
zarf tools k9s
```

### Clean Up

``` bash
# Destroy the k3d cluster
k3d cluster delete
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
