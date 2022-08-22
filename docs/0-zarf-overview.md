import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";

# Overview

![Zarf Underwater](.images/Zarf%20Left%20Underwater%20-%20Behind%20rock.svg)

## What is Zarf?

Zarf is an open-source tool that simplifies the setup & deployment of applications and resources onto AirGap or disconnected environments. You can think of a disconnected environment as a system that has the has limited connection to the internet, kind of like airplane mode.

Zarf equips you with the ability to quickly and securely deploy modern software onto these types of systems without relying on internet connectivity. It also simplifies the installation, updating, and maintenance of DevSecOps capabilities like Kubernetes clusters, logging, and SBOM compliance out of the box. Most importantly Zarf keeps applications and systems running even when they are disconnected.

## How Zarf works?

Zarf uses a static Go binary CLI that can be run on any machine, with or without internet connectivity. The Zarf CLI equips users with the ability to pull, package, and install all the resources their applications or clusters need to run without being connected to the internet. It can also deploy any necessary resources needed to stand up infrastructure tools (such as Terraform).

All that is needed to deploy your infrastructure, application, and resources in a disconnected environment is 3 files; the Zarf CLI binary, the Zarf init package, and a Zarf Package containing your app and resources.

![Zarf CLI + Zarf Init + Zarf Package](.images/Zarf%20Files%20-%20%203%20Bubbles.svg)

:::note

For more information on how zarf works under the hood visit our [Nerd Notes page](./6-developer-guide/3-nerd-notes.md)

:::

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
