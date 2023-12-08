---
sidebar_position: 1
---

# Quick Start

1. 💻 Select your system's OS below.
2. ❗ Ensure you have the pre-requisite applications running.
3. `$` Enter the commands into your terminal.

<Tabs>
<TabItem value="Linux">

:::info

This quick start requires you to already have:

- [Homebrew](https://brew.sh/) package manager installed on your machine.
- [Docker](https://www.docker.com/) installed and running on your machine.

For more install options please visit our [Getting Started page](./2-getting-started/index.md).

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

For more install options please visit our [Getting Started page](./2-getting-started/index.md).

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

There is currently no Zarf Quick Start for Windows, though you can learn how to install Zarf from our Github Releases by visiting the [Getting Started page](./2-getting-started/index.md#downloading-from-github-releases)

:::

```text

Coming soon!

```

</TabItem>
</Tabs>

Zarf is being actively developed by the community. For more information, see our [release notes](https://github.com/defenseunicorns/zarf/releases).
