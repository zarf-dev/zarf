## Zarf Appliance Mode Example

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no gitops service and Zarf is simply a standard means of wrapping airgap concerns for K3s. 

# Zarf Data Injection Example

This example deploys a basic K3s cluster using Traefik 2 and configures TLS / airgap concerns to deploy [Podinfo](https://github.com/stefanprodan/podinfo).
## The Flow

Here's what you'll do in this example:

1. [Get ready](#get-ready)

1. [Create a cluster](#create-a-cluster)

1. [Package it](#package-it)

1. [Deploy it](#deploy-it)

1. [Try it](#try-it)

1. [Cleanup](#cleanup)

&nbsp;

## Get ready

Before the magic can happen you have to do a few things:

1. Install [Docker](https://docs.docker.com/get-docker/). Other container engines will likely work as well but aren't actively tested by the Zarf team.

2. Install [KinD](https://github.com/kubernetes-sigs/kind). Other Kubernetes distros will work as well, but we'll be using KinD for this example since it is easy and tested frequently and thoroughly.

3. Clone the Zarf project &mdash; for the example configuration files.

4. Download a Zarf release &mdash; you need a binary _**and**_ an init package, [here](../../docs/workstation.md#just-gimmie-zarf).

&nbsp;

## Create a cluster

You can't run software without _somewhere to run it_, so the first thing to do is create a local Kubernetes cluster that Zarf can deploy to. In this example we'll be using KinD to create a lightweight, local K8s cluster running in Docker.

Kick that off by running this command:

```sh
kind create cluster
```

This will result in a single-node Kubernetes cluster called `kind-kind` on your local machine running in Docker. Your KUBECONFIG should be automatically configured to talk to the new cluster.

```sh
cd <same dir as zarf-init-<arch>.tar.zst>
zarf init
```

Follow the prompts, answering "no" to each of the optional components, since we don't need them for this deployment.

Congratulations! Your machine is now running a single-node Kubernetes cluster powered by Zarf!

> _**Note**_
>
> Zarf supports non-interactive installs too! Give `zarf init --confirm --components logging` a try next time.

**Troubleshooting:**

> _**ERROR: Unable to find the package on the local system, expected package at zarf-init-<arch>.tar.zst**_
>
> The zarf binary needs an init package to know how to setup your cluster! So, if `zarf init` returns an error like this:
>
> ```sh
> ERROR:  Unable to find the package on the local system, expected package at zarf-init-<arch>.tar.zst
> ```
>
> It's likely you've either forgotten to download `zarf-init-<arch>.tar.zst` (as part of [getting ready](#get-ready)) _**OR**_ you are _not_ running `zarf init` from the directory the init package is sitting in.

> _**ERROR: failed to create cluster: node(s) already exist for a cluster with the name "kind"**_
>
> You already have a KinD cluster running. Either just move on to use the current cluster, or run `kind delete cluster`, then `kind create cluster`.

> _**Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?**_
>
> Docker isn't running or you're otherwise not able to talk to it. Check your Docker installation, then try again.

&nbsp;

## Package it

Zarf is (at heart) a tool for making it easy to get software from _where you have it_ to _**where you need it**_&mdash;specifically, across an airgap. Since moving bits is so core to Zarf the idea of a "ready-to-move group of software" has a specific name&mdash;the _package_.

All of the software a Zarf cluster runs is installed via package&mdash;for many reasons like versioning, auditability, etc&mdash;which means that if you want to run a in your cluster you're going to have to build a package for it.

Luckily, this is very easy to do&mdash;package contents are defined by simple, declarative yaml files and _we've already made one for you_. To build this package you simply:

```sh
cd <zarf dir>/examples/data-injection     # directory with zarf.yaml, and
zarf package create --confirm             # make the package
```

Watch the terminal scroll for a while. Once things are downloaded & zipped up and you'll see a file ending in `.tar` drop. _That's_ your package.  

*This package ends in .tar instead of .tar.zst because the `zarf.yaml` uncrompressed flag is set to true.*

&nbsp;

## Deploy it

It's time to feed the package you built into your cluster.

Since you're running a Zarf cluster directly on your local machine&mdash;where this package & `zarf` binary _already are_&mdash;deploying the package is very simple:

```sh
zarf package deploy zarf-package-data-injection-demo-<arch>.tar --confirm
```

In a couple seconds the cluster will have loaded your package.

&nbsp;

## Use it

This demo should have placed some test files in the cluster from the zarf package.  To verify they were created, you can run the following command:

```shell
kubectl exec -n demo data-injection -- cat /test/this-is-an-example-file.txt 
```

The output should say:
>This is a sample file to be injected into the cluster.  Normal flow would keep this data gitignored as it would likely be large.


&nbsp;

## Cleanup

Once you've had your fun it's time to clean up.

In this case, since the Zarf cluster was installed specifically (and _only_) to serve this example, clean up is really easy&mdash;you just tear down the entire cluster:

```sh
kind delete cluster
```

It only takes a couple moments for the _entire cluster_ to disappear&mdash;long-running system services and all&mdash;leaving your machine ready for the next adventure.

&nbsp;
