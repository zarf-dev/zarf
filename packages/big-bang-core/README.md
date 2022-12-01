# Big Bang Core

This package deploys [Big Bang Core](https://repo1.dso.mil/platform-one/big-bang/bigbang) using Zarf.

![pods](./images/pods.png)

![helmreleases](./images/helmreleases.png)

## Known Issues

- Currently this package does the equivalent of `kustomize build | kubectl apply -f -`, which means Flux will be used to deploy everything, but it won't be watching a Git repository for changes. Upcoming work is planned to update the package so that you will be able to open up a Git repo in the private Gitea server inside the cluster, commit and push a change, and see that change get reflected in the deployment.

> NOTE:
> Big Bang requires an AMD64 system to deploy as Iron Bank does not yet support ARM.  You will need to deploy to a cluster that is running AMD64.  Specifically, M1 Apple computers are not supported locally and you will need to provision a remote cluster to work with Big Bang currently.

## Instructions

### Pull down the code and binaries [local machine]

```shell
# Clone the binaries
git clone https://github.com/defenseunicorns/zarf.git

# Change dir
cd zarf
```

### Get Zarf components [local machine]

Follow instructions on [this page](../../docs/5-operator-manual/0-set-up-and-install.md) to get the `zarf` cli and the `zarf-init*.tar.zst` package and place them in the ./build directory

Alternatively, you could build the components from the repo
```shell
# Build zarf components from scratch (NOTE: golang and npm must be installed)
make init-package
```

### Build the deploy package [local machine]

```shell
# Change dir
cd packages/big-bang-core

# Authenticate to the registry with Big Bang artifacts
set +o history
export REGISTRY1_USERNAME=<REPLACE_ME>
export REGISTRY1_PASSWORD=<REPLACE_ME>
echo $REGISTRY1_PASSWORD | ../../build/zarf tools registry login registry1.dso.mil --username $REGISTRY1_USERNAME --password-stdin
set -o history

# Run zarf package command
../../build/zarf package create . --confirm
```

### Start the Vagrant VM [local machine]

```shell
# Change dir back to top of repo
cd ../../

# Setup ~/.vagrant.d/Vagrantfile to mount packages dir
file=~/.vagrant.d/Vagrantfile; test -f $file || mkdir -p "$(dirname "${file}")" && cp -pfv "${file}" "${file}.orig" 2>/dev/null; echo -en 'Vagrant.configure(2) do |config|\n config.vm.synced_folder "packages/", "/usr/local/src/zarf-packages", mount_options: ["uid=0", "gid=0"]\nend\n' > $file

# Start the VM
make vm-init OS=ubuntu

# Shell into the VM
vagrant ssh ubuntu
```

### Initialize Zarf [ubuntu vm]

```shell
# Switch to root user and change dir
sudo su -
cd /opt/zarf

# Initialize Zarf
/opt/zarf/zarf init --confirm --components k3s,git-server

# (Optional) Inspect the results
/opt/zarf/zarf tools k9s
```

### Deploy Big Bang [ubuntu vm]

```shell
# Deploy Big Bang (lightweight version)
cd /usr/local/src/zarf-packages/big-bang-core
/opt/zarf/zarf package deploy --confirm $(ls -1 zarf-package-big-bang-core-demo-amd64*.tar.zst) --components big-bang-core-limited-resources
# NOTE: you can deploy the standard full set of components using the flag:
# '--components big-bang-core-standard'

# (Optional) Inspect the results
/opt/zarf/zarf tools k9s
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
