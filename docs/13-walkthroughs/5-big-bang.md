# Using Big Bang with Zarf

## Introduction

This walkthrough describes how to use Big Bang with Zarf for Air Gap deployments through the use of the Big Bang Zarf extension. If you are not familiar with Big Bang you can learn more about it here: https://p1.dso.mil/products/big-bang, but in short it is a DevSecOps platform that contains many useful tools for building, managing, and running software projects while adhering to the [United States Department of Defense DevSecOps Reference Design](https://public.cyber.mil/devsecops/).

Zarf integrates with Big Bang through the use of an extension that simplifies the selection of Big Bang packages and the pulling of the required artifacts to deploy those packages in an Air Gap.

### Known Issues

The current version of this extension works best with Big Bang version `1.54.0` or later, and is not fully integrated into the `zarf package remove` lifecycle.  We will be looking to remove these limitations in a future release.

## System Requirements

Big Bang requires a reasonably powerful `amd64` system that scales up with the number of components deployed.  We recommend at least `32 GB` of RAM and a high-speed internet connection to complete this walkthrough.

To learn more about Big Bang's requirements in general, see their documentation: https://docs-bigbang.dso.mil/latest/docs/prerequisites/minimum-hardware-requirements/

## Pre-requisites

Before beginning this walkthrough you will need the following:

- A local copy of the Zarf repository
  - `git clone https://github.com/defenseunicorns/zarf.git`
- A kubernetes cluster onto which you can deploy Zarf and Big Bang
  - Follow steps 3 through 5 of the [Big Bang Quick Start](https://docs-bigbang.dso.mil/1.54.0/docs/guides/deployment-scenarios/quickstart/) to setup `docker` and a `k3d` cluster.
- The latest version of the Zarf `cli`
  - Follow instructions on https://zarf.dev/install/
- An account on `https://registry1.dso.mil` to retrieve Big Bang images
  - You can register for an account [here](https://login.dso.mil/auth/realms/baby-yoda/protocol/openid-connect/registrations?client_id=account&response_type=code)

:::note

Big Bang requires some additional [configuration options](https://docs-bigbang.dso.mil/1.54.0/docs/guides/deployment-scenarios/quickstart/#Explanation-of-k3d-Command-Flags-Relevant-to-the-Quick-Start) to be passed to `k3d` than is required in most other Zarf walkthroughs.  Below are some examples:

```bash
  # Required by the PLG stack
  --volume /etc/machine-id:/etc/machine-id

  # Required for Istio ingress
  --k3s-arg "--disable=traefik@server:0"
  --port 80:80@loadbalancer
  --port 443:443@loadbalancer

  # Required for TLS to work correctly with kubectl
  --k3s-arg "--tls-san=$SERVER_IP@server:0"
  --api-port 6443
```

If you tweak the packages that are deployed there may be other configuration options you need to specify, please refer to the [Big Bang documentation](https://docs-bigbang.dso.mil/1.54.0/docs/) for more details.

:::

## Package Creation

The below section covers creating and tuning the Big Bang package:

### Setup

By default, Big Bang uses images from [Iron Bank](https://p1.dso.mil/products/iron-bank) which will require you to set your login credentials for [Registry One](https://registry1.dso.mil) (see [pre-requisites](#pre-requisites) for information on account setup).

```bash
# Authenticate to https://registry1.dso.mil/, then retrieve your CLI secret from your User Profile and run the following:
set +o history
export REGISTRY1_USERNAME=<REPLACE_ME>
export REGISTRY1_CLI_SECRET=<REPLACE_ME>
echo $REGISTRY1_CLI_SECRET | zarf tools registry login registry1.dso.mil --username $REGISTRY1_USERNAME --password-stdin
set -o history
```

Now navigate to the `examples/big-bang` folder within the Zarf repository you cloned in the [pre-requisites](#pre-requisites) section.

### Configure Big Bang

Within the `examples/big-bang` folder you will see a `zarf.yaml` that has the following [component](../4-user-guide/2-zarf-packages/2-zarf-components.md) defined:

```yaml
components:
  - name: bigbang
    required: true
    extensions:
      bigbang:
        version: 1.54.0
        skipFlux: false
        valuesFiles:
          - config/minimal.yaml #turns on just istio
          - config/ingress.yaml # adds istio certs for *.bigbang.dev
          - config/kyverno.yaml # turns on kyverno
          - config/loki.yaml # turns on loki and monitoring
```

This component uses the `bigbang` extension to define the version of Big Bang to use and the values files to apply.  Feel free to inspect and configure the values.yaml files as you wish and to learn more about Big Bang's configuration see their values guide: https://docs-bigbang.dso.mil/latest/docs/guides/using-bigbang/values-guide/

:::note

The `valuesFiles` are applied from top to bottom and will apply the last value that was provided for any given key.

:::

:::note

This extension works best with Big Bang version `1.54.0` or later.  Version `1.53.0` requires manual patches to images to function correctly.

:::


### Package Big Bang

When you're ready to continue you can create a Big Bang package by running the following command in `examples/big-bang`:

```bash
zarf package create
```

Now wait for the package creation to complete and you should see a `zarf-package-big-bang-example-amd64-x.x.x.tar.zst` file in the directory.


## Package Deployment

The below section covers deploying the Big Bang package from the previous section:

### Initialize Zarf

Before you can deploy the Big Bang package you must first initialize Zarf on the cluster you created in the [pre-requisites](#pre-requisites) section.  To do so you can run the following:

```bash
# Initialize Zarf (interactively)
zarf init
# Make these choices at the prompts
# ? Do you want to download this init package? Yes
# ? Deploy this Zarf package? Yes
# ? Deploy the k3s component? No
# ? Deploy the logging component? No
# ? Deploy the git-server component? Yes

# (Optional) Inspect the results
zarf tools k9s
```

:::note

The `git-server` component is required by Big Bang as it uses it as a source for Flux deployments.

:::


### Deploy Big Bang

Now you are ready to deploy Big Bang, and can do so with the following in the `examples/big-bang` directory:

```bash
# Deploy Big Bang (interactively)
zarf package deploy
# Make these choices at the prompts
# ? Choose or type the package file [tab for suggestions] zarf-package-big-bang-example-amd64-x.x.x.tar.zst
# ? Deploy this Zarf package? Yes
```

### See The Results

Once the install completes you can inspect the results and watch the Big Bang components deploy using the following:

```bash
zarf tools k9s

# To view different k8s objects you can use the following:

# Helm Releases:
# :hr [Enter]
# Pods:
# :pods [Enter]
# Services:
# :svc [Enter]
# Secrets:
# :secret [Enter]
# ConfigMaps:
# :configmap [Enter]

# When you are done use the following to quit
# :q [Enter]
```

## Package Removal

The Big Bang package is not fully integrated into the Zarf package remove lifecycle (see [known issues](#known-issues)), but for the purposes of this walkthrough you can simply remove your k3d cluster:

```bash
k3d cluster delete
```

## Troubleshooting

See the Troubleshooting section of the Big Bang Quick Start for help troubleshooting the Big Bang deployment: https://repo1.dso.mil/big-bang/bigbang/-/blob/master/docs/guides/deployment-scenarios/quickstart.md#troubleshooting

Also, ensure that you have followed all of the steps required in the [pre-requisites](#pre-requisites) section and have reviewed the [known issues](#known-issues).

If you feel that the error you are encountering is one with Zarf feel free to [open an issue](https://github.com/defenseunicorns/zarf/issues/new/choose) or reach out via [slack](https://kubernetes.slack.com/archives/C03B6BJAUJ3).
