import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";

# The Zarf CLI

<!-- TODO: @JPERRY This text seems a bit short, What else can we be saying here? -->
<!-- TODO: @JPERRY Is mentioning Cobra actually useful here? -->
<!-- TODO: @JPERRY Should I mention the OS and arch when talking about statically built binaries? -->

Zarf is a command line interface (CLI) tool that enables secure software delivery, with a particular focus on delivery to disconnected or complex environments. Zarf is a statically compiled Go binary, which means it can be utilized in any environment without requiring additional dependencies. The Zarf CLI project is an always free, open-source project available on [GitHub](https://github.com/defenseunicorns/zarf).

## Getting the CLI

<!-- TODO: @JPERRY Is it better to link to 'Installing Zarf' or should we repeat the information here? (check w/ Madeline) -->
<!-- TODO: @JPERRY Make sure the 'Installing Zarf' section if fully up to date with all the installation methods -->

You can get the Zarf CLI on your machine in a few different ways. You can use the Defense Unicorns Homebrew Tap, download a prebuilt binary from our GitHub releases, or build the CLI from scratch on your own. We provide instructions for all of these methods in the [Installing Zarf](../1-getting-started/index.md#installing-zarf) section of the Getting Started guide. If you're eager to start using Zarf and you already have Homebrew installed, you can quickly install it by copying and pasting the relevant commands for your operating system into your terminal:

<!-- NOTE: The empty line after the '<TabItem ...>' lines are important for the rendering... -->
<Tabs>
<TabItem value="macOS">

```bash
brew tap defenseunicorns/tap
brew install zarf
```

</TabItem>

<TabItem value="Linux">

```bash
brew tap defenseunicorns/tap
brew install zarf
```

</TabItem>
</Tabs>

## Verify the CLI

<!-- TODO: @JPERRY A lot of this stuff could (and probably should) go in the 'Installing Zarf' section -->

To begin, we'll test whether the CLI you have is functioning correctly. Running the CLI will generate a help message output, which will verify its functionality. Depending on the method you used to install the CLI, the tabs below will provide guidance on how to initiate it for the first time. Upon successful installation, you should see a comprehensive list of all command options, along with concise descriptions of their functions.

<details><summary>Expected Help Output</summary>
<p>
The output of the help command should look <b>something</b> like this (CLI flags will also appear at the end of the output):

```text
Zarf eliminates the complexity of air gap software delivery for Kubernetes clusters and cloud native workloads
using a declarative packaging strategy to support DevSecOps in offline and semi-connected environments.

Usage:
  zarf [COMMAND]|[ZARF-PACKAGE]|[ZARF-YAML] [flags]
  zarf [command]

Available Commands:
  completion        Generate the autocompletion script for the specified shell
  connect           Access services or pods deployed in the cluster
  destroy           Tear it all down, we'll miss you Zarf...
  help              Help about any command
  init              Prepares a k8s cluster for the deployment of Zarf packages
  package           Zarf package commands for creating, deploying, and inspecting packages
  prepare           Tools to help prepare assets for packaging
  tools             Collection of additional tools to make airgap easier
  version           Displays the version of the Zarf binary
```

</p>
</details>

<Tabs>
<TabItem value="Installed with Homebrew">

```bash
zarf --help
```

</TabItem>

<TabItem value="Downloaded from Github">

- If you're not sure where the file was downloaded, a good default place to look is `~/Downloads`.
- While we only say `zarf` for this example command, the name of the binary is the name of the file you downloaded, which will likely have a different name.

```bash
chmod +x ~/Downloads/zarf   # Make the binary executable
~/Downloaded/zarf --help
```

</TabItem>

<TabItem value="Manually Built">

- While we only say `zarf` for this example command, depending on your system, you might have to use a different name for the binary like `zarf-mac-intel` or `zarf-mac-apple`.

```bash
cd ./path/to/zarf/repo
cd build
./zarf --help
```

</TabItem>

</Tabs>

### Adding The CLI To Your Path

:::note
If you installed Zarf through Homebrew, Zarf will already be on your $PATH and you can skip this section.
:::

To simplify the usage of the Zarf CLI, you may add it to your $PATH. This configuration will allow you to use `zarf` without having to specify the binary's precise location and your computer will automatically find the binary for you to execute. The directories listed in your $PATH can be viewed by executing the command `echo $PATH` in your terminal. If you move your CLI to any of these directories, you will be able to execute it without the need to specify its full path. A typical $PATH you can use is: `mv ./path/to/cli/file/zarf /usr/local/bin/zarf`

:::note
Throughout the rest of the documentation, we will often be describing commands as `zarf {command}`. This assumes that the CLI is on your $PATH.
:::

## Introduction to Zarf Commands

Zarf provides a suite of commands that streamline the creation, deployment, and maintenance of packages. Some of these commands contain additional sub-commands to further assist with package management. When executed with the "--help" flag, each command and sub-command provides a concise summary of its functionality. As you navigate deeper into the command hierarchy, the provided descriptions become increasingly detailed. We encourage you to explore the various commands available to gain a comprehensive understanding of Zarf's capabilities.

As previously mentioned, Zarf was specifically designed to facilitate the deployment of applications in disconnected environments with ease. As a result, the most commonly utilized commands are `zarf init`, `zarf package create`, and `zarf package deploy`. Detailed information on all commands can be found in the [CLI Commands](./100-cli-commands/zarf.md) section. However, brief descriptions of the most frequently used commands are provided below. It's worth noting that these three commands are closely linked to what we refer to as a "Zarf Package". Additional information on Zarf Packages can be found in the following section: [Zarf Packages](../3-create-a-zarf-package/1-zarf-packages.md).

### zarf init

<!-- TODO: Find a good place to talk about what the init command is actually doing (there's a lot of special magic sauce going on with that command) -->

The `zarf init` command is utilized to configure a K8s cluster in preparation for the deployment of future Zarf Packages. The init command uses a specialized 'init-package' to operate. This package may be located in your current working directory, the directory where the Zarf CLI binary is located, or downloaded from GitHub releases during command execution. For further details regarding the init-package, please refer to the [init-package](../3-create-a-zarf-package/3-zarf-init-package.md) page.

### zarf package deploy

<!-- The most common use case (like 99.9% of the time) is deploying onto a k8s cluster.. but that doesn't HAVE to be the case.. How do I write the docs for this then? -->
<!-- TODO: Write some docs (or redirect to other docs) describing when you would be able to do a `zarf package deploy` before a `zarf init` -->

The `zarf package deploy` command is used to deploy an already built tar.zst package onto a machine, typically within a K8s cluster. Generally, it is presumed that the `zarf init` command has already been executed on the target machine. However, there are a few exceptional cases where this assumption does not apply.
