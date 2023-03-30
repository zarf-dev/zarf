# Getting Started

Welcome to the Zarf documentation! This section will list the various ways to install Zarf onto your machine. It will also demonstrate how to verify the installation. Choose the installation type that best suits your needs in accordance with your operating system. Let’s get started.

## Installing Zarf

There are multiple ways to get the Zarf CLI onto your machine:

- Install from [Homebrew](getting-started#installing-from-the-defense-unicorns-homebrew-tap).
- [Download a binary](https://github.com/defenseunicorns/zarf/tags).
- [Build the CLI](#building-the-cli-from-scratch) from scratch.

### Installing from the Defense Unicorns Homebrew Tap

[Homebrew](https://brew.sh/) is an open-source software package manager that simplifies the installation of software on macOS and Linux. With Homebrew, installing Zarf is simple:

```bash
brew tap defenseunicorns/tap
brew install zarf
```

The above command detects your OS and system architecture and installs the correct Zarf CLI binary for your machine. Once the above command is entered, the CLI should be installed on your $PATH and ready for immediate use.

### Downloading a Prebuilt Binary from our GitHub Releases

All [Zarf releases](https://github.com/defenseunicorns/zarf/releases) on GitHub include prebuilt binaries that you can download and use. We offer a small range of combinations of OS and architecture for you to choose from. Once downloaded, you can install the binary onto your $PATH by moving the binary to the `/usr/local/bin` directory:

```bash
mv ./path/to/downloaded/{ZARF_FILE} /usr/local/bin/zarf
```

### Building the CLI from Scratch

If you want to build the CLI from scratch, you can do that too. Our local builds depend on [Go 1.19.x](https://golang.org/doc/install) and are built using [make](https://www.gnu.org/software/make/).

:::note
The `make` build-cli` command builds a binary for each combination of OS and architecture. If you want to shorten the build time, you can use an alternative command to only build the binary you need:

- `make build-cli-mac-intel`
- `make build-cli-mac-apple`
- `make build-cli-linux-amd`
- `make build-cli-linux-arm`

For additional information, see the [Building Your Own Zarf CLI](./4-user-guide/1-the-zarf-cli/1-building-your-own-cli.md) page.
:::

---

## Verifying Zarf Install

Now that you have installed Zarf, let's verify that it is working. First, we'll check the version of Zarf that has been installed:

```bash
$ zarf version

vX.X.X  # X.X.X is replaced with the version number of your specific installation
```

If you are not seeing this then Zarf was not installed onto your $PATH correctly. [This $PATH guide](https://zwbetz.com/how-to-add-a-binary-to-your-path-on-macos-linux-windows/) should help with that.

---

## Where to Next?

Depending on how familiar you are with Kubernetes, DevOps, and Zarf, let's find what set of information would be most useful to you.

- If you want to become more familiar with Zarf and it's features, see the [Walkthroughs](./13-walkthroughs/index.md) page.

- More information about the Zarf CLI is available on the [Zarf CLI](./4-user-guide/1-the-zarf-cli/index.md) page, or by browsing through the help descriptions of all the commands available through `zarf --help`.

- More information about the packages that Zarf creates and deploys is available in the [Understanding Zarf Packages](./4-user-guide/2-zarf-packages/1-zarf-packages.md) page.

- If you want to take a step back and better understand the problem Zarf is trying to solve, you can find more context on the [Understand the Basics](./1-understand-the-basics.md) and [Core Concepts](./2-core-concepts.md) pages.
