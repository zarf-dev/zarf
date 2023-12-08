import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";

# Getting Started

Welcome to the Zarf documentation! This section will list the various ways to install Zarf onto your machine. It will also demonstrate how to verify the installation. Choose the installation type that best suits your needs in accordance with your operating system. Let's get started!

## Quick Start

Trying out Zarf is as simple as:

1. 💻 Selecting your system's OS below.
2. ❗ Ensuring you have the pre-requisite applications running.
3. `$` Entering the commands into your terminal.

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
zarf package deploy oci://defenseunicorns/dos-games:1.0.0-$(uname -m) --key=https://zarf.dev/cosign.pub
```

:::note

This example shows how to install Zarf with the official (📜) `defenseunicorns` Homebrew tap, however there are many other options to install Zarf on Linux such as:

- 📜 **[official]** Downloading Zarf directly from [GitHub releases](https://github.com/defenseunicorns/zarf/releases)
- 🧑‍🤝‍🧑 **[community]** `apk add` on [Alpine Linux Edge](https://pkgs.alpinelinux.org/package/edge/testing/x86_64/zarf)
- 🧑‍🤝‍🧑 **[community]** `asdf install` with the [ASDF Version Manager](https://github.com/defenseunicorns/asdf-zarf)
- 🧑‍🤝‍🧑 **[community]** `nix-shell`/`nix-env` with [Nix Packages](https://search.nixos.org/packages?channel=23.05&show=zarf&from=0&size=50&sort=relevance&type=packages&query=zarf)

:::

:::tip

Zarf can deploy it's own `k3s` cluster on Linux if you have `root` access by selecting the `k3s` component on `zarf init`.

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
zarf package deploy oci://🦄/dos-games:1.0.0-$(uname -m) --key=https://zarf.dev/cosign.pub
```

</TabItem>
<TabItem value="Windows">

## Windows Commands


:::info

There is currently no Zarf Quick Start for Windows, though you can learn how to install Zarf from our Github Releases by visiting the [Getting Started page](./1-getting-started/index.md#downloading-from-github-releases)

:::

```text

Coming soon!

```

</TabItem>
</Tabs>

Zarf is being actively developed by the community. For more information, see our [release notes](https://github.com/defenseunicorns/zarf/releases).

## Where to Next?

Depending on how familiar you are with Kubernetes, DevOps, and Zarf, let's find what set of information would be most useful to you.

- If you want to become more familiar with Zarf and it's features, see the [Tutorials](../5-zarf-tutorials/index.md) page.

- More information about the Zarf CLI is available on the [Zarf CLI](../2-the-zarf-cli/index.md) page, or by browsing through the help descriptions of all the commands available through `zarf --help`.

- More information about the packages that Zarf creates and deploys is available in the [Understanding Zarf Packages](../3-create-a-zarf-package/1-zarf-packages.md) page.

- If you want to take a step back and better understand the problem Zarf is trying to solve, you can find more context on the [Understand the Basics](./0-understand-the-basics.md) and [Core Concepts](./1-core-concepts.md) pages.
