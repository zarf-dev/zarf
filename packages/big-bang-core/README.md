# Big Bang Core

This package deploys [Big Bang Core](https://repo1.dso.mil/platform-one/big-bang/bigbang) using Zarf.

![pods](./images/pods.png)

![helmreleases](./images/helmreleases.png)

## Known Issues

Lots of new things
* dynamic finding of images
* Hard coding patches
* can only use one values.yaml file
* update docs here to use a binary instead of the go function.

## Instructions

### Pull down the code and binaries

```shell
# clone the binaries
git clone https://github.com/defenseunicorns/zarf.git

# change to the examples folder
cd zarf/examples/big-bang-core

```

### Build the deploy package

```shell
# Create the deploy package and move it to the 'examples/sync' folder
go run ../../main.go package create
```

### Deploy an EKS cluster

```shell
eksctl create cluster -f eksctl/demo.yaml
```

Now wait 20 min :face_palm:

### Initialize Zarf

```shell
# Initialize Zarf
go run ../../main.go init -a amd64 --confirm --components git-server

# (Optional) Inspect the results
./zarf tools k9s
```

### Deploy Big Bang

```shell
# Deploy Big Bang
./zarf package deploy zarf-package-big-bang-core-demo-arm64-1.47.0.tar.zst --confirm

# (Optional) Inspect the results
./zarf tools k9s
```

### See the results

```shell
kubectl get pods -n flux-system
kubectl get hr -n bigbang
kubectl get pods -A
```


### Clean Up

```shell
# Inside the VM
eksctl delete cluster -f eksctl/demo.yaml --disable-nodegroup-eviction --wait
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
