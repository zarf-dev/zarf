# The Zarf CLI

Zarf is a command line interface (CLI) tool that enables secure software delivery, with a particular focus on delivery to disconnected or highly regulated environments. Zarf is a statically compiled Go binary, which means it can be utilized in any environment without requiring additional dependencies.

## Getting the CLI

You can get the Zarf CLI on your machine in a few different ways, using the Defense Unicorns Homebrew Tap, downloading a prebuilt binary from our GitHub releases, or building the CLI from scratch on your own.

We provide instructions for all of these methods in the [Installing Zarf](../1-getting-started/index.md#installing-zarf) section of the Getting Started guide.

## Introduction to Zarf Commands

Zarf provides a suite of commands that streamline the creation, deployment, and maintenance of packages. Some of these commands contain additional sub-commands to further assist with package management. When executed with the `--help` flag, each command and sub-command provides a concise summary of its functionality. As you navigate deeper into the command hierarchy, the provided descriptions become increasingly detailed. We encourage you to explore the various commands available to gain a comprehensive understanding of Zarf's capabilities.

As previously mentioned, Zarf was specifically designed to facilitate the deployment of applications in disconnected environments with ease. As a result, the most commonly utilized commands are `zarf init`, `zarf package create`, and `zarf package deploy`. Detailed information on all commands can be found in the [CLI Commands](./100-cli-commands/zarf.md) section. However, brief descriptions of the most frequently used commands are provided below. It's worth noting that these three commands are closely linked to what we refer to as a "Zarf Package". Additional information on Zarf Packages can be found on the [Zarf Packages](../3-create-a-zarf-package/1-zarf-packages.md) page.

### zarf init

The `zarf init` command is used to configure a K8s cluster in preparation for the deployment of future Zarf Packages. The init command uses a specialized 'init-package' to operate which may be located in your current working directory, the directory where the Zarf CLI binary is located, or downloaded from the GitHub Container Registry during command execution. For further details regarding the init-package, please refer to the [init-package](../3-create-a-zarf-package/3-zarf-init-package.md) page.

### zarf package deploy

The `zarf package deploy` command is used to deploy an already created Zarf package onto a machine, typically to a K8s cluster. Generally, it is presumed that the `zarf init` command has already been executed on the target machine, however, there are a few exceptional cases where this assumption does not apply.  You can learn more about deploying Zarf packages on the [Deploy a Zarf Package](../4-deploy-a-zarf-package/index.md) page.

:::tip

When deploying and managing packages you may find the sub-commands under `zarf tools` useful to troubleshoot or interact with deployments.

:::

### zarf package create

The `zarf package create` command is used to create a Zarf package from a `zarf.yaml` package definition.  This command will pull all of the defined resources into a single package you can take with you to a disconnected environment.  You can learn more about creating Zarf packages on the [Create a Zarf Package](../3-create-a-zarf-package/index.md) page.

:::tip

When developing packages you may find the sub-commands under `zarf dev` useful to find resources and manipulate package definitions.

:::
