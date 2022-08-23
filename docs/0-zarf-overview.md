import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";

# Overview

![Zarf Underwater](.images/Zarf%20Left%20Underwater%20-%20Behind%20rock.svg)

## What is Zarf?

Zarf was created to _**support the declarative creation & distribution of software "packages" into remote / constrained / independent environments**_.

> "Zarf is a tool to help deploy modern stacks into air gapped environments; it's all about moving the bits." &mdash; Jeff

Zarf is a free and open-source tool that simplifies the setup & deployment of applications and resources onto AirGap or disconnected environments. Zarf equips you with the ability to quickly and securely deploy modern software onto these types of systems without relying on internet connectivity.

It also simplifies the installation, updating, and maintenance of DevSecOps capabilities like Kubernetes clusters, logging, and SBOM compliance out of the box. Most importantly Zarf keeps applications and systems running even when they are disconnected.

\* Check out our [glossary](1-understand-the-basics.md) for an explantion of common terms used in the project.

## How Zarf works?

Zarf simplifies and standardizes the delivery of complex deployments. Giving users the ability to reduce tens / hundreds of individual software updates, movements, and manual installations to a few simple terminal commands. The tool equips users with the ability to pull, package, and install all the resources their applications or clusters needs to run without being connected to the internet. It can also deploy any necessary resources needed to stand up infrastructure tools (such as Terraform).

![Zarf CLI + Zarf Init + Zarf Package](.images/Zarf%20Files%20-%20%203%20Bubbles.svg)

A typical Zarf deployment is made up of three parts

1. The `zarf` binary:

- a statically compiled Go binary that can be run on any machine, server, or operating system with or without connectivity.
- creates packages containing numerous software types / updates into a single distributable package (while on an internet-accessible network)
- declaratively deploys package contents "into place" for use on production systems (while on an internet-isolated network).

1. A Zarf init package

- compressed tarball package that contains the configuration needed to instantiate an environment without connectivity
- Automatically seeds your cluster with a container registry
- Provide additional capabilities such as (logging, git server, K8 cluster)
<!-- Per Jon, need validation on if putting K8 after seeds registry is confusing -->

3. A Zarf Package

- compressed tarball package that contains all of the files, manifests, repos, and images needed to deploy your infrastructure, application, and resources in a disconnected environment.

:::note

For more information on how zarf works under the hood visit our [Nerd Notes page](./6-developer-guide/3-nerd-notes.md)

:::

## Target Use Cases

Zarf's possibilities are endless&mdash;Zarf developers' time is not. Thus, scope definition is in order.

Here are the things we think Zarf should get really good at, listed top-down in order of importance:

- Make movement of software "across the air gap" an open source "solved problem".

- Make it trivial to deploy & run Kubernetes apps "at the Edge".

- Make it easy to support GitOps-based k8s cluster updates in isolated environments.

- Make it possible to support GitOps-based k8s cluster updates in internet-connected-but-independent environments (think: dependency caching per availability zone, etc).

&nbsp;

## What can be packaged?

To reiterate: Zarf's possibilities are endless&mdash;Zarf developers' time is not. Thus, scope definition is again in order.

Given Zarf's being a "k8s cluster to serve _other_ k8s clusters", the following types of software can be rolled into a Zarf package:

- container images &mdash; to serve images for the Zarf & downstream clusters to run containers from.

- git repositories &mdash; to serve as the git-based "source of truth" for downstream "GitOps"ed k8s clusters to watch.

- pre-compiled binaries &mdash; to provide the software necessary to start & support the Zarf cluster.

## How To Use Zarf

Zarf is intended for use in a software deployment process that looks something like this:

<a href="../.images/what-is-zarf/how-to-use-it.png">
  <img alt="how it works" src="../.images/what-is-zarf/how-to-use-it.png" heigth="262" />
</a>

### (0) - Connect to Internet

Zarf doesn't build software—it helps you distribute software that already exists.

Zarf can pull from lots of places like Docker Hub, Iron Bank, GitHub, local filesystems, etc. but you have to make sure that Zarf has a clear path & appropriate access credentials. Be sure you know what you want pack & how to access it before you begin using Zarf.

### (1) - Create a Package

This part of the process requires access to the internet. You feed the `zarf` binary a "recipe" (`zarf.yaml`) and it makes itself busy downloading, packing, and compressing the software you asked for. It outputs a single, ready-to-move distributable (cleverly) called "a package".

Find out more about what that looks like in the [Building a package](.//13-walkthroughs/0-creating-a-zarf-package.md) section.

### (2) - Ship the Package to system location

When it comes to remote / constrained / independent / air gapped systems, everyone's unique. Zarf doesn't have an opinion as to _how_ packages move just so long as you can get them into your downstream environment.

### (3) - Deploy the package

Once your package has landed you will need to:

- install the binary onto the system,
- run the you have run the zarf init package
- deploy your package to your cluster.

## Cluster Configuration Options

Zarf allows the package to either deploy to an existing K8's cluster or can spin up a local cluster (K3s) to deploy you package to. This is a configuration that is available on deployment in the init package.

### Appliance Cluster Mode

![Appliance Mode Diagram](../.images/what-is-zarf/appliance-mode.png)

In the simplest usage scenario, your package consists of a single application (plus dependencies) and you configure the Zarf cluster to serve your application directly to end users. This mode of operation is called "Appliance Mode"— because it's small & self-contained like a kitchen appliance—and it is intended for use in environments where you want to run k8s-native tooling but need to keep a small footprint (i.e. single-purpose / constrained / "edge" environments).

### Utility Cluster Mode

![Appliance Mode Diagram](../.images/what-is-zarf/utility-mode.png)

In the more complex use case, your package consists of updates for many apps / systems and you configure the Zarf cluster to propagate updates to downstream systems rather than to serve users directly. This mode of operation is called "Utility Mode"—as it's main job is to add utility to other clusters—and it is intended for use in places where you want to run independent, full-service production environments (ex. your own Big Bang cluster) but you need help tracking, caching & disseminating system / dependency updates.

## Why Use Zarf?

- 💸 **Free and Open Source.** Zarf will always be free to use and maintained by the open source community.
- 🔓 **No Vender Lock.** There is no proprietary software that locks you into using Zarf. If you want to remove it, you still can use your help charts to deploy your software manually.
- 💻 **OS Agnostic.** Zarf supports numerous operating systems.For a full list, visit the [Supported OSes](./5-operator-manual/90-supported-oses.md) page.
- 📦 **Highly Distributable.** Integrate and deploy software from multiple, secure development environments including edge, embedded systems, secure cloud, data centers, and even local environments.
- 🚀 **Develop Connected Deploy Disconnected.** Teams can build, and configure individual applications or entire DevSecOps environments while connected to the internet and then package and ship them to a disconnected environment to be deployed.
- 💿 **Single File Deployments.** Zarf allows you to package the parts of the internet your app needs into a single compressed file to be installed without connectivity.
- ♻️ **Declarative Deployments.**
- 🦖 **Inherit Legacy Code**

## Features

### 📦 Out of the Box Features

- Automate Kubernetes deployments in disconnected environments
- Automate [Software Bill of Materials (SBOM)](https://www.linuxfoundation.org/tools/the-state-of-software-bill-of-materials-sbom-and-cybersecurity-readiness/) generation
- Provide a [web dashboard](https://docs.zarf.dev/docs/dashboard-ui/sbom-dashboard) for viewing SBOM output
- Deploy a new cluster while fully disconnected with [K3s](https://k3s.io/) or into any existing cluster using a [kube config](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
- Builtin logging stack with [Loki](https://grafana.com/oss/loki/)
- Builtin git server with [Gitea](https://gitea.com/)
- Builtin docker registry
- Builtin [K9s Dashboard](https://k9scli.io/) for managing a cluster from the terminal
- [Mutating Webhook](adr/0005-mutating-webhook.md) to automatically update Kubernetes pods image path and pull secrets as well as [Flux Git Repository](https://fluxcd.io/docs/components/source/gitrepositories/) URLs and secret references
- Builtin [command to find images](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/cli-commands/zarf_prepare_find-images) and resources from a helm chart
- Tunneling capability to [connect to Kuberenetes resources](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/cli-commands/zarf_connect) without network routing, DNS, TLS or Ingress configuration required

### 🛠️ Configurable Features

- Customizable [packages variables](examples/package-variables/README.md) with defaults and user prompting
- [Composable packages](https://docs.zarf.dev/docs/user-guide/zarf-packages/zarf-components#composing-package-components) to include multiple sub-packages/components
- Filters to select the correct architectures/operating systems for packages

## Quick Start

:::info

This quick start requires you to already have [home brew](https://brew.sh/) package manager installed on your machine.
For more install options please visit our [Getting Started page](3-getting-started.md)

:::

To download the Zarf CLI Binary,

1.  Select your systems OS below
2.  copy and past the quick start command into your computers terminal.

<Tabs>
<TabItem value="macOS" label="macOS" default>

```bash
brew tap defenseunicorns/tap
brew install zarf
```

</TabItem>
<TabItem value="Linux" label="Linux">

```bash
brew tap defenseunicorns/tap
brew install zarf
```

</TabItem>
<TabItem value="Windows" label="Windows">

```bash
Coming Soon!
```

</TabItem>
</Tabs>

Zarf is being activity developed by the community. Our releases can be found [here](https://github.com/defenseunicorns/zarf/releases).
