---
sidebar_position: 0
---

import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";

# Overview

![Zarf Underwater](.images/Zarf%20Left%20Underwater%20-%20Behind%20rock.svg)

## What is Zarf?

Zarf was created to _**support the declarative creation & distribution of software "packages" into remote/constrained/independent environments**_.

> "Zarf is a tool to help deploy modern stacks into air-gapped environments; it's all about moving the bits." &mdash; Jeff

Zarf is a free and open-source tool that simplifies the setup and deployment of applications and resources onto air-gapped or disconnected environments. Zarf equips you with the ability to quickly and securely deploy modern software onto complex systems without relying on internet connectivity.

It also simplifies the installation, updating, and maintenance of DevSecOps capabilities like Kubernetes clusters, logging, and SBOM compliance out of the box. Most importantly, Zarf keeps applications and systems running even when they are disconnected.

:::note

Check out our [glossary](1-getting-started/0-understand-the-basics.md) for an explanation of common terms used in the project.

:::

## How Zarf Works

Zarf simplifies and standardizes the delivery of complex software deployments. This gives users the ability to reduce tens/hundreds of individual software updates, movements, and manual installations to a few simple terminal commands. This tool equips users with the ability to pull, package, and install all the resources their applications or clusters need to run without being connected to the internet. It can also deploy any necessary resources needed to stand up infrastructure tools (such as Terraform).

![Zarf CLI + Zarf Init + Zarf Package](.images/Zarf%20Files%20-%20%203%20Bubbles.svg)

A typical Zarf deployment is made up of three parts:

1. The `zarf` binary:
   - A statically compiled Go binary that can be run on any machine, server, or operating system with or without connectivity.
   - Creates packages containing numerous software types/updates into a single distributable package (while on an internet-accessible network).
   - Declaratively deploys package contents "into place" for use on production systems (while on an internet-isolated network).
2. A Zarf init package:
   - A compressed tarball package that contains the configuration needed to instantiate an environment without connectivity.
   - Automatically seeds your cluster with a container registry.
   - Provides additional capabilities such as logging, git server, and K8s cluster.
3. A Zarf Package:
   - A compressed tarball package that contains all of the files, manifests, source repositories, and images needed to deploy your infrastructure, application, and resources in a disconnected environment.

:::note

For more technical information on how Zarf works and to view the Zarf architecture, visit our [Nerd Notes page](./12-contribute-to-zarf/3-nerd-notes.md).

:::

## Target Use Cases

- Make the delivery of software "across the air gap" an open-source "solved problem".
- Make it trivial to deploy and run Kubernetes apps "at the Edge".
- Make it easy to support GitOps-based K8s cluster updates in isolated environments.
- Make it possible to support GitOps-based K8s cluster updates in internet-connected-but-independent environments (think: dependency caching per availability zone, etc).

## What can be Packaged?

Given Zarf's being a "K8s cluster to serve _other_ K8s clusters", the following types of software can be rolled into a Zarf Package:

- Container images: to serve images for the Zarf and downstream clusters to run containers from.
- Repositories: to serve as the git-based "source of truth" for downstream "GitOps"ed K8s clusters to watch.
- Pre-compiled binaries: to provide the software necessary to start and support the Zarf cluster.
- [Component actions](3-create-a-zarf-package/7-component-actions.md): to support scripts and commands that run at various stages of the Zarf [package create lifecycle](./3-create-a-zarf-package/5-package-create-lifecycle.md), and [package deploy lifecycle](./4-deploy-a-zarf-package/1-package-deploy-lifecycle.md).
- Helm charts, kustomizations, and other K8s manifests: to apply in a Kubernetes cluster.
- [Data injections](../examples/kiwix/README.md): to declaratively inject data into running containers in a Kubernetes cluster.

## How To Use Zarf

Zarf is intended for use in a software deployment process that looks similar to this:

![How Zarf works](./.images/what-is-zarf/how-to-use-it.png)

### (0) Connect to the Internet

Zarf doesn't build software—it helps you distribute software that already exists.

Zarf can pull from various places like Docker Hub, Iron Bank, GitHub, and local filesystems. In order to do this, you must ensure that Zarf has a clear path and appropriate access credentials. Be sure you know what you want to pack and how to access it before you begin using Zarf.

### (1) Create a Package

This part of the process requires access to the internet. The `zarf` binary is presented with a `zarf.yaml`, it then begins downloading, packing, and compressing the software that you requested. It then outputs a single, ready-to-move distributable called "a package".

For additional information, see the [Creating a package](./5-zarf-tutorials/0-creating-a-zarf-package.md) section.

### (2) Ship the Package to the System Location

Zarf enables secure software delivery for various environments, such as remote, constrained, independent, and air-gapped systems. Considering there are various target environments with their own appropriate transferring mechanisms, Zarf does not determine _how_ packages are moved so long as they can arrive in your downstream environment.

### (3) Deploy the Package

Once your package has arrived, you will need to:

1. Install the binary onto the system.
2. Run the zarf init package.
3. Deploy the package to your cluster.

## Cluster Configuration Options

Zarf allows the package to either deploy to an existing K8s cluster or a local K3s cluster. This is a configuration that is available on deployment in the init package.

### Appliance Cluster Mode

![Appliance Mode Diagram](.images/what-is-zarf/appliance-mode.png)

In the simplest usage scenario, your package consists of a single application (plus dependencies) and you configure the Zarf cluster to serve your application directly to end users. This mode of operation is called "Appliance Mode" and it is intended for use in environments where you want to run K8s-native tooling but need to keep a small footprint (i.e. single-purpose/constrained/"Edge" environments).

### Utility Cluster Mode

![Appliance Mode Diagram](.images/what-is-zarf/utility-mode.png)

In the more complex use case, your package consists of updates for many apps/systems and you configure the Zarf cluster to propagate updates to downstream systems rather than to serve users directly. This mode of operation is called "Utility Mode" and it is intended for use in places where you want to run independent, full-service production environments (ex. your own Big Bang cluster) but you need help tracking, caching and disseminating system/dependency updates.

## Why Use Zarf?

- 💸 **Free and Open-Source.** Zarf will always be free to use and maintained by the open-source community.
- ⭐️ **Zero Dependencies.** As a statically compiled binary, the Zarf CLI has zero dependencies to run on any machine.
- 🔓 **No Vendor Lock.** There is no proprietary software that locks you into using Zarf. If you want to remove it, you still can use your helm charts to deploy your software manually.
- 💻 **OS Agnostic.** Zarf supports numerous operating systems. A full matrix of supported OSes, architectures and featuresets is coming soon.
- 📦 **Highly Distributable.** Integrate and deploy software from multiple secure development environments including edge, embedded systems, secure cloud, data centers, and even local environments.
- 🚀 **Develop Connected, Deploy Disconnected.** Teams can build and configure individual applications or entire DevSecOps environments while connected to the internet. Once created, they can be packaged and shipped to a disconnected environment to be deployed.
- 💿 **Single File Deployments.** Zarf allows you to package the parts of the internet your app needs into a single compressed file to be installed without connectivity.
- ♻️ **Declarative Deployments.** Zarf packages define the precise state for your application enabling it to be deployed the same way every time.
- 🦖 **Inherit Legacy Code.** Zarf packages can wrap legacy code and projects - allowing them to be deployed to modern DevSecOps environments.

## Features

<!-- mirrored from the project's README.md -->

### 📦 Out of the Box Features

- Automate Kubernetes deployments in disconnected environments
- Automate [Software Bill of Materials (SBOM)](./3-create-a-zarf-package/6-package-sboms.md) generation
- Build and [publish packages as OCI image artifacts](./5-zarf-tutorials/7-publish-and-deploy.md)
- Provide a [web dashboard](./4-deploy-a-zarf-package/4-view-sboms.md) for viewing SBOM output
- Create and verify package signatures with [cosign](https://github.com/sigstore/cosign)
- [Publish](./2-the-zarf-cli/100-cli-commands/zarf_package_publish.md), [pull](./2-the-zarf-cli/100-cli-commands/zarf_package_pull.md), and [deploy](./2-the-zarf-cli/100-cli-commands/zarf_package_deploy.md) packages from an [OCI registry](https://opencontainers.org/)
- Powerful component lifecycle [actions](./3-create-a-zarf-package/7-component-actions.md)
- Deploy a new cluster while fully disconnected with [K3s](https://k3s.io/) or into any existing cluster using a [kube config](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
- Builtin logging stack with [Loki](https://grafana.com/oss/loki/)
- Builtin Git server with [Gitea](https://gitea.com/)
- Builtin Docker registry
- Builtin [K9s Dashboard](https://k9scli.io/) for managing a cluster from the terminal
- [Mutating Webhook](adr/0005-mutating-webhook.md) to automatically update Kubernetes pod's image path and pull secrets as well as [Flux Git Repository](https://fluxcd.io/docs/components/source/gitrepositories/) URLs and secret references
- Builtin [command to find images](./2-the-zarf-cli/100-cli-commands/zarf_prepare_find-images.md) and resources from a Helm chart
- Tunneling capability to [connect to Kuberenetes resources](./2-the-zarf-cli/100-cli-commands/zarf_connect.md) without network routing, DNS, TLS or Ingress configuration required

### 🛠️ Configurable Features

- Customizable [variables and package templates](examples/variables/README.md) with defaults and user prompting
- [Composable packages](./3-create-a-zarf-package/2-zarf-components.md#composing-package-components) to include multiple sub-packages/components
- Component-level OS/architecture filtering

## Quick Start

1. 💻 Select your system's OS below.
2. ❗ Ensure you have the pre-requisite applications running.
3. `$` Enter the commands into your terminal.

<Tabs>
<TabItem value="Linux">

:::info

This quick start requires you to already have:

- [Homebrew](https://brew.sh/) package manager installed on your machine.
- [Docker](https://www.docker.com/) installed and running on your machine.

For more install options please visit our [Getting Started page](./1-getting-started/index.md).

:::

## Linux Commands

```bash
# To install Zarf
brew tap defenseunicorns/tap && brew install zarf

# Next, you will need a Kubernetes cluster. This example uses KIND.
brew install kind && kind delete cluster && kind create cluster


# Then, you will need to deploy the Zarf Init Package
zarf init


# You are ready to deploy any Zarf Package, try out our Retro Arcade!!
zarf package deploy oci://ghcr.io/defenseunicorns/packages/dos-games:1.0.0-$(uname -m) --key=https://zarf.dev/cosign.pub
```

:::note

Zarf has no prerequisites on Linux. However, for this example, we will use Docker and Kind.

:::

</TabItem>
<TabItem value="macOS">

:::info

This quick start requires you to already have:

- [Homebrew](https://brew.sh/) package manager installed on your machine.
- [Docker](https://www.docker.com/) installed and running on your machine.

For more install options please visit our [Getting Started page](./1-getting-started/index.md).

:::

## MacOS Commands

```bash
# To install Zarf
brew tap defenseunicorns/tap && brew install zarf

# Next, you will need a Kubernetes cluster. This example uses KIND.
brew install kind && kind delete cluster && kind create cluster


# Then, you will need to deploy the Zarf Init Package
zarf init


# You are ready to deploy any Zarf Package, try out our Retro Arcade!!
zarf package deploy oci://ghcr.io/defenseunicorns/packages/dos-games:1.0.0-$(uname -m) --key=https://zarf.dev/cosign.pub
```

</TabItem>
<TabItem value="Windows">

## Windows Commands

```text
Coming soon!
```

</TabItem>
</Tabs>

Zarf is being actively developed by the community. For more information, see our [release notes](https://github.com/defenseunicorns/zarf/releases).
