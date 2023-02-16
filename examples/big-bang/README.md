# Big Bang Core

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using Zarf.

## Known Issues

Lots of new things
* dynamic finding of images
* Hard coding patches

## Instructions

### Clone this repo or download this folder

``` bash
# Clone the binaries
git clone https://github.com/defenseunicorns/zarf.git

# change to the examples folder
cd zarf/examples/big-bang

```

### Deploy a Kubernetes Cluster

Any Kubernetes cluster that supports AMD64 architecture will work (Big Bang uses Iron Bank, which has no support for ARM). The following instructions are for deploying a cluster on your local machine using `k3d`.

#### Deploy k3d Cluster

Follow instructions on [this page](../../docs/13-walkthroughs/index.md#walk-through-prerequisites) for docker and the `k3d` cli.

### Get Zarf components

Follow instructions on  https://zarf.dev/install/ to get the `zarf` cli

### Build the deploy package

``` bash
# Authenticate to the registry with Big Bang artifacts, https://registry1.dso.mil/
set +o history
export REGISTRY1_USERNAME=<REPLACE_ME>
export REGISTRY1_PASSWORD=<REPLACE_ME>
echo $REGISTRY1_PASSWORD | zarf tools registry login registry1.dso.mil --username $REGISTRY1_USERNAME --password-stdin
set -o history

# Run zarf package command and follow the prompts
zarf package create .
```

Now wait a while... 😩

### Initialize Zarf

``` bash
# Initialize Zarf (interactively)
zarf init
# Make these choices at the prompt
# ? Do you want to download this init package? Yes
# ? Deploy this Zarf package? Yes
# ? Deploy the k3s component? No
# ? Deploy the logging component? No
# ? Deploy the git-server component? Yes

# (Optional) Inspect the results
zarf tools k9s
```

### Configure and Package BigBang

Look at the values files provided to BigBang in the Zarf.yaml:

```yaml
components:
  - name: bigbang
    required: true
    extensions:
      bigbang:
        version: 1.52.0
        skipFlux: false
        valuesFrom:
          - config/minimal.yaml #turns on just istio
          - config/ingress.yaml # adds istio certs for *.bigbang.dev
          - config/kyverno.yaml # turns on kyverno
          - config/loki.yaml # turns on loki and monitoring
          - config/patch-images.yaml # fixes upstream bug in BB

```

And adjust them to how you want BigBang to be deployed.  When you're ready, package BigBang:

```shell

zarf package create

```


### Deploy Big Bang

```shell
# Deploy Big Bang (interactively)
./zarf package deploy 

# (Optional) Inspect the results
zarf tools k9s
```

### See the results

```shell
kubectl get pods -n flux-system
kubectl get hr -n bigbang
kubectl get pods -A
```


### Clean Up


#### K3d

```shell
# Destroy the k3d cluster
k3d cluster delete

```

## Troubleshooting

### My computer crashed!
Close all those hundreds of chrome tabs, shut down all non-essential programs, and try again. Big Bang is a HOG. If you have less than 32GB of RAM you're in for a rough time and should use a remote cluster.
