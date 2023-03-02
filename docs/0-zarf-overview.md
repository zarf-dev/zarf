import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";
import Admonition from "@theme/Admonition";

# Overview

![Zarf Underwater](.images/Zarf%20Left%20Underwater%20-%20Behind%20rock.svg)

## What is Zarf?

Zarf was created to _**support the declarative creation & distribution of software "packages" into remote/constrained/independent environments**_.

> "Zarf is a tool to help deploy modern stacks into air gapped environments; it's all about moving the bits." &mdash; Jeff

Zarf is a free and open-source tool that simplifies the setup and deployment of applications and resources onto AirGap or disconnected environments. Zarf equips you with the ability to quickly and securely deploy modern software onto these types of systems without relying on internet connectivity.

It also simplifies the installation, updating, and maintenance of DevSecOps capabilities like Kubernetes clusters, logging, and SBOM compliance out of the box. Most importantly Zarf keeps applications and systems running even when they are disconnected.

\* Check out our [glossary](1-understand-the-basics.md) for an explanation of common terms used in the project.

## How Zarf works?

Zarf simplifies and standardizes the delivery of complex deployments. Giving users the ability to reduce tens/hundreds of individual software updates, movements, and manual installations to a few simple terminal commands. The tool equips users with the ability to pull, package, and install all the resources their applications or clusters need to run without being connected to the internet. It can also deploy any necessary resources needed to stand up infrastructure tools (such as Terraform).

![Zarf CLI + Zarf Init + Zarf Package](.images/Zarf%20Files%20-%20%203%20Bubbles.svg)

A typical Zarf deployment is made up of three parts:

1. The `zarf` binary:
   - A statically compiled Go binary that can be run on any machine, server, or operating system with or without connectivity.
   - Creates packages containing numerous software types/updates into a single distributable package (while on an internet-accessible network)
   - Declaratively deploys package contents "into place" for use on production systems (while on an internet-isolated network).
2. A Zarf init package:
   - A compressed tarball package that contains the configuration needed to instantiate an environment without connectivity
   - Automatically seeds your cluster with a container registry
   - Provide additional capabilities such as (logging, git server, and K8s cluster)
3. A Zarf Package:
   - A compressed tarball package that contains all of the files, manifests, source repositories, and images needed to deploy your infrastructure, application, and resources in a disconnected environment.

:::note

For more technical information on how Zarf works and to view the Zarf architecture visit our [Nerd Notes page](./6-developer-guide/3-nerd-notes.md)

:::

## Target Use Cases

- Make the delivery of software "across the air gap" an open-source "solved problem".
- Make it trivial to deploy and run Kubernetes apps "at the Edge".
- Make it easy to support GitOps-based K8s cluster updates in isolated environments.
- Make it possible to support GitOps-based K8s cluster updates in internet-connected-but-independent environments (think: dependency caching per availability zone, etc).

## What can be packaged?

Given Zarf's being a "K8s cluster to serve _other_ k8s clusters", the following types of software can be rolled into a Zarf package:

- Container images: to serve images for the Zarf and downstream clusters to run containers from.
- Repositories: to serve as the git-based "source of truth" for downstream "GitOps"ed K8s clusters to watch.
- Pre-compiled binaries: to provide the software necessary to start and support the Zarf cluster.
- [Component actions](4-user-guide/7-github-action.md): to support scripts and commands that run at various stages of the Zarf [component lifecycle](4-user-guide/4-package-command-lifecycle.md)
- Helm charts, kustomizations, and other k8s manifests: to apply in a Kubernetes cluster
- [Data injections](../examples/data-injection/README.md): to declaratively inject data into running containers in a Kubernetes cluster

## How To Use Zarf

Zarf is intended for use in a software deployment process that looks something like this:

![How Zarf works](./.images/what-is-zarf/how-to-use-it.png)

### (0) - Connect to the Internet

Zarf doesn't build software—it helps you distribute software that already exists.

Zarf can pull from lots of places like Docker Hub, Iron Bank, GitHub, local filesystems, etc. but you have to make sure that Zarf has a clear path & appropriate access credentials. Be sure you know what you want to pack & how to access it before you begin using Zarf.

### (1) - Create a Package

This part of the process requires access to the internet. You feed the `zarf` binary a `zarf.yaml` "recipe" and it makes itself busy downloading, packing, and compressing the software you asked for. It outputs a single, ready-to-move distributable (cleverly) called "a package".

Find out more about what that looks like in the [Building a package](./13-walkthroughs/0-using-zarf-package-create.md) section.

### (2) - Ship the Package to the system location

When it comes to remote, constrained, independent, air-gapped systems, everyone's unique. Zarf doesn't have an opinion as to _how_ packages move just so long as you can get them into your downstream environment.

### (3) - Deploy the package

Once your package has landed you will need to:

- install the binary onto the system,
- run the zarf init package
- deploy your package to your cluster.

## Cluster Configuration Options

Zarf allows the package to either deploy to an existing K8s cluster or a local K3s cluster. This is a configuration that is available on deployment in the init package.

### Appliance Cluster Mode

![Appliance Mode Diagram](.images/what-is-zarf/appliance-mode.png)

In the simplest usage scenario, your package consists of a single application (plus dependencies) and you configure the Zarf cluster to serve your application directly to end users. This mode of operation is called "Appliance Mode"— because it's small & self-contained like a kitchen appliance—and it is intended for use in environments where you want to run k8s-native tooling but need to keep a small footprint (i.e. single-purpose/constrained/"edge" environments).

### Utility Cluster Mode

![Appliance Mode Diagram](.images/what-is-zarf/utility-mode.png)

In the more complex use case, your package consists of updates for many apps/systems and you configure the Zarf cluster to propagate updates to downstream systems rather than to serve users directly. This mode of operation is called "Utility Mode"—as its main job is to add utility to other clusters—and it is intended for use in places where you want to run independent, full-service production environments (ex. your own Big Bang cluster) but you need help tracking, caching & disseminating system/dependency updates.

## Why Use Zarf?

- 💸 **Free and Open-Source.** Zarf will always be free to use and maintained by the open-source community.
- 🔓 **No Vendor Lock.** There is no proprietary software that locks you into using Zarf. If you want to remove it, you still can use your helm charts to deploy your software manually.
- 💻 **OS Agnostic.** Zarf supports numerous operating systems. For a full list, visit the [Supported OSes](./5-operator-manual/90-supported-oses.md) page.
- 📦 **Highly Distributable.** Integrate and deploy software from multiple, secure development environments including edge, embedded systems, secure cloud, data centers, and even local environments.
- 🚀 **Develop Connected Deploy Disconnected.** Teams can build, and configure individual applications or entire DevSecOps environments while connected to the internet and then package and ship them to a disconnected environment to be deployed.
- 💿 **Single File Deployments.** Zarf allows you to package the parts of the internet your app needs into a single compressed file to be installed without connectivity.
- ♻️ **Declarative Deployments.**
- 🦖 **Inherit Legacy Code**

## Features

### 📦 Out of the Box Features

- Automates Kubernetes deployments in disconnected environments
- Automates [Software Bill of Materials (SBOM)](https://www.linuxfoundation.org/tools/the-state-of-software-bill-of-materials-sbom-and-cybersecurity-readiness/) generation
- Provides an [SBOM dashboard UI](dashboard-ui/sbom-dashboard)
- Deploys a new cluster while fully disconnected with [K3s](https://k3s.io/) or into any existing cluster using a [Kube config](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
- Built-in logging stack with [Loki](https://grafana.com/oss/loki/)
- Built-in git server with [Gitea](https://gitea.com/)
- Built-in docker registry
- Built-in [K9s Dashboard](https://k9scli.io/) for managing a cluster from the terminal
- [Mutating Webhook](adr/0005-mutating-webhook.md) to automatically update Kubernetes pod's image path and pull secrets as well as [Flux Git Repository](https://fluxcd.io/docs/components/source/gitrepositories/) URLs and secret references
- Built-in [command to find images](user-guide/the-zarf-cli/cli-commands/zarf_prepare_find-images) and resources from a helm chart
- Tunneling capability to [connect to Kubernetes resources](user-guide/the-zarf-cli/cli-commands/zarf_connect) without network routing, DNS, TLS, or Ingress configuration required

### 🛠️ Configurable Features

- Customizable [packages variables](examples/package-variables/README.md) with defaults and user prompting
- [Composable packages](user-guide/zarf-packages/zarf-components#composing-package-components) to include multiple sub-packages/components
- Filters to select the correct architectures/operating systems for packages

## Quick Start

1. 💻 Select your system's OS below
2. ❗ Ensure you have the pre-requisite applications running
3. `$` Enter the commands into your terminal

<Tabs>
<TabItem value="Linux" label="Linux">
  
<Admonition type="info">

This quick start requires you to already have:

- [Homebrew](https://brew.sh/) package manager installed on your machine.
- [Docker](https://www.docker.com/) is installed and running on your machine
  For more install options please visit our [Getting Started page](3-getting-started.md)

</Admonition>

## Linux Commands

```bash
# To install Zarf
brew tap defenseunicorns/tap brew install zarf

# Next, you will need a Kubernetes cluster. This example uses KIND.
brew install kind && kind delete cluster && kind create cluster


# Then, you will need to deploy the Zarf Init Package
zarf init


# You are ready to deploy any Zarf Package, try out our Retro Arcade!!
zarf package deploy sget://defenseunicorns/zarf-hello-world:$(uname -m)
```

<Admonition type="note">

Zarf has no prerequisites on Linux. However, for this example, we will use Docker and Kind.

</Admonition>

</TabItem>
<TabItem value="macOS" label="macOS">

<Admonition type="info">

This quick start requires you to already have:

- [Homebrew](https://brew.sh/) package manager installed on your machine.
- [Docker](https://www.docker.com/) is installed and running on your machine
  For more install options please visit our [Getting Started page](3-getting-started.md)

</Admonition>

## MacOS Commands

```bash
# To install Zarf
brew tap defenseunicorns/tap brew install zarf

# Next, you will need a Kubernetes cluster. This example uses KIND.
brew install kind && kind delete cluster && kind create cluster


# Then, you will need to deploy the Zarf Init Package
zarf init


# You are ready to deploy any Zarf Package, try out our Retro Arcade!!
zarf package deploy sget://defenseunicorns/zarf-hello-world:$(uname -m)
```

</TabItem>
<TabItem value="Windows" label="Windows">

## Windows Commands

```text
Coming soon!
```

</TabItem>
</Tabs>

Zarf is being actively developed by the community. Our releases can be found [here](https://github.com/defenseunicorns/zarf/releases).
