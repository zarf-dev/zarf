# Getting Started

Welcome to the Zarf documentation! This section will list the various ways to install Zarf onto your machine. It will also demonstrate how to verify the installation. Choose the installation type that best suits your needs in accordance with your operating system. Let's get started!

## Installing Zarf

There are multiple ways to get the Zarf CLI onto your machine:

- [Install from Homebrew](#installing-from-the-defense-unicorns-homebrew-tap).
- [Download a prebuilt binary](#downloading-a-prebuilt-binary-from-our-github-releases).
- [Build the CLI](#building-the-cli-from-scratch) from scratch.

### Installing with Homebrew

[Homebrew](https://brew.sh/) is an open-source software package manager that simplifies the installation of software on macOS and Linux.


<Tabs>
<TabItem value="macOS">

With Homebrew on macOS, installing Zarf is as simple as:

```bash
brew tap defenseunicorns/tap && brew install zarf
```

</TabItem>
<TabItem value="Linux">

With Homebrew on Linux, installing Zarf is as simple as:

```bash
brew tap defenseunicorns/tap && brew install zarf
```

</TabItem>

:::note

The above command detects your OS and system architecture and installs the correct Zarf CLI binary for your machine. Once the above command is entered, the CLI should be installed on your `$PATH` and is ready for immediate use.

:::

### Downloading from  GitHub Releases

All [Zarf releases](https://github.com/defenseunicorns/zarf/releases) on GitHub include prebuilt binaries that you can download and use. We offer range of combinations of OS and architecture for you to choose from.

<Tabs>
<TabItem value="Linux">

To download Zarf on Linux you can run the following (replacing `<zarf-version>` with a version of Zarf):

```bash
ZARF_VERSION=<zarf-version>
ZARF_ARCH=$([ $(uname -m) == "x86_64" ] && echo "amd64" || echo "arm64";)

curl -sL https://github.com/defenseunicorns/zarf/releases/download/${ZARF_VERSION}/zarf_${ZARF_VERSION}_Linux_${ZARF_ARCH} -o zarf
chmod +x zarf
```

On most Linux distributions, you can also install the binary onto your `$PATH` by simply moving the downloaded binary to the `/usr/local/bin` directory:

```bash
sudo mv zarf /usr/local/bin/zarf
```

</TabItem>
<TabItem value="macOS">

To download Zarf on macOS you can run the following (replacing `<zarf-version>` with a version of Zarf):

```bash
ZARF_VERSION=<zarf-version>
ZARF_ARCH=$([ $(uname -m) == "x86_64" ] && echo "amd64" || echo "arm64";)

curl -sL https://github.com/defenseunicorns/zarf/releases/download/${ZARF_VERSION}/zarf_${ZARF_VERSION}_Darwin_${ZARF_ARCH} -o zarf
chmod +x zarf
```

You can also install the binary onto your `$PATH` by simply moving the downloaded binary to the `/usr/local/bin` directory:

```bash
sudo mv zarf /usr/local/bin/zarf
```

</TabItem>
<TabItem value="Windows">


To download Zarf on Windows you can run the following (replacing `<zarf-version>` with a version of Zarf and `<zarf-arch>` with either `amd64` or `arm64` depending on your system):

```powershell
$ZarfVersion="<zarf-version>"
$ZarfArch="<zarf-arch>"

Start-BitsTransfer -Source "https://github.com/defenseunicorns/zarf/releases/download/$($ZarfVersion)/zarf_$($ZarfVersion)_Windows_$($ZarfArch).exe" -Destination zarf.exe
```

You can also install the binary onto your `$PATH` by moving the downloaded binary to the desired directory and modifying the `$PATH` environment variable to include that directory.

</TabItem>
</Tabs>

### Building the CLI from Scratch

If you want to build the CLI from scratch, you can do that too. Our local builds depend on [Go 1.19.x](https://golang.org/doc/install) and [Node 18.x](https://nodejs.org/en) and are built using [make](https://www.gnu.org/software/make/).

:::note

The `make build-cli` command builds a binary for each combination of OS and architecture. If you want to shorten the build time, you can use an alternative command to only build the binary you need:

- `make build-cli-mac-intel`
- `make build-cli-mac-apple`
- `make build-cli-linux-amd`
- `make build-cli-linux-arm`
- `make build-cli-windows-amd`
- `make build-cli-windows-arm`

For additional information, see the [Building Your Own Zarf CLI](../2-the-zarf-cli/0-building-your-own-cli.md) page.

:::

## Verifying the Zarf Install

Now that you have installed Zarf, let's verify that it is working. First, we'll check the version of Zarf that has been installed:

```bash
$ zarf version

vX.X.X  # X.X.X is replaced with the version number of your specific installation
```

If you are not seeing this then Zarf was not installed onto your `$PATH` correctly. [This $PATH guide](https://zwbetz.com/how-to-add-a-binary-to-your-path-on-macos-linux-windows/) should help with that.

---

## Downloading the ['Init' Package](../3-create-a-zarf-package/3-zarf-init-package.md)

The ['init' package](../3-create-a-zarf-package/3-zarf-init-package.md) is a special Zarf package that initializes a cluster with services that are used to store resources while in the air gap and is required for most ([but not all](../../examples/yolo/README.md)) Zarf deployments.

You can get it for your version of Zarf by visiting the [Zarf releases](https://github.com/defenseunicorns/zarf/releases) page and downloading it into your working directory or into `~/.zarf-cache/zarf-init-<amd64|arm64>-vX.X.X.tar.zst`)

If you are online on the machine with cluster access you can also run `zarf init` without the `--confirm` flag to be given the option to download the version of the init package for your Zarf version or you can use the `zarf tools download-init` command.

:::note

You can build your own custom 'init' package too if you'd like. For this you should check out the [Creating a Custom 'init' Package Tutorial](../5-zarf-tutorials/8-custom-init-packages.md).

:::

---

## Where to Next?

Depending on how familiar you are with Kubernetes, DevOps, and Zarf, let's find what set of information would be most useful to you.

- If you want to become more familiar with Zarf and it's features, see the [Tutorials](../5-zarf-tutorials/index.md) page.

- More information about the Zarf CLI is available on the [Zarf CLI](../2-the-zarf-cli/index.md) page, or by browsing through the help descriptions of all the commands available through `zarf --help`.

- More information about the packages that Zarf creates and deploys is available in the [Understanding Zarf Packages](../3-create-a-zarf-package/1-zarf-packages.md) page.

- If you want to take a step back and better understand the problem Zarf is trying to solve, you can find more context on the [Understand the Basics](./0-understand-the-basics.md) and [Core Concepts](./1-core-concepts.md) pages.
