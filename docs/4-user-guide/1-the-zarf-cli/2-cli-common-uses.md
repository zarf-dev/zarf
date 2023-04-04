# Common CLI Uses

Zarf is a tool that optimizes the delivery of applications and capabilities into various environments, starting with air-gapped systems. This is achieved by using Zarf Packages, which are declarative files that the Zarf CLI uses to create, deploy, inspect, and remove applications and capabilities.

## Building Packages: `zarf package create`

To create a Zarf Package, you must execute the [`zarf package create`](./100-cli-commands/zarf_package_create.md) command, which generates a `tar.zst` archive that includes all the required dependencies and instructions to deploy the capabilities onto another machine. The `zarf package create` command uses a `zarf.yaml` configuration file that describes the package's components and performs all necessary actions, such as downloading container images and git repositories, to build the final package. Additional information on Zarf Packages can be found on the [Zarf Packages](../2-zarf-packages/index.md) page.

## Initializing a Cluster: `zarf init`

<!-- TODO: Find a good place to talk about what the init command is doing (there's a lot of special magic sauce going on with that command) -->
<!-- TODO: Should we talk about the 'Zarf Agent - A Mutating Webhook' here? -->

Before deploying a package to a cluster, you must initialize the cluster using the [`zarf init`](./100-cli-commands/zarf_init.md) command. This command creates and bootstraps an in-cluster container registry and provides the option to install optional tools and services necessary for future packages. 

For Windows and macOS environments, a cluster must already exist before initializing it using Zarf. You can use [Kind](https://kind.sigs.k8s.io/), [K3d](https://k3d.io/), [Docker Desktop](https://docs.docker.com/desktop/kubernetes/), or any other local or remote Kubernetes cluster. 

For Linux environments, Zarf can, itself, create and update a local K3s cluster, in addition to using any other local or remote Kubernetes cluster. The init package used by `zarf init` contains all the resources necessary to create a local [K3s](https://k3s.io/) cluster on your machine. This package may be located in your current working directory, the directory where the Zarf CLI binary is located, or downloaded from GitHub releases during command execution. Further details on the initialization process can be found on the [init package](../2-zarf-packages/3-the-zarf-init-package.md) page.

:::note
Depending on the permissions of your user, if you are installing K3s with `zarf init`, you may need to run it as a privileged user. This can be done by either:

- Becoming a privileged user via the command `sudo su` and then running all the Zarf commands as you normally would.
- Manually running all the Zarf commands as a privileged user via the command `sudo <command>`.
- Running the init command as a privileged user via `sudo zarf init` and then changing the permissions of the `~/.kube/config` file to be readable by the current user.
:::

## Deploying Packages: `zarf package deploy`

<!-- TODO: Write some docs (or redirect to other docs) describing when you would be able to do a `zarf package deploy` before a `zarf init` -->

The [`zarf package deploy`](./100-cli-commands/zarf_package_deploy.md) command deploys the packaged capabilities into the target environment. The package can be deployed on any cluster, even those without an external internet connection, since it includes all of its external resources. The external resources are pushed into the cluster to services Zarf either deployed itself or that it was told about on `init`, such as the init package's Gitea Git server or a pre-existing Harbor image registry.  Then, the application is deployed according to the instructions in the zarf.yaml file, such as deploying a helm chart, deploying raw K8s manifests, or executing a series of shell commands. Generally, it is presumed that the `zarf init` command has already been executed on the target machine. However, there are a few exceptional cases where this assumption does not apply, such as [YOLO Mode](../../9-faq.md#what-is-yolo-mode-and-why-would-i-use-it).

Additional information about Zarf Packages can found on the [Understanding Zarf Packages](../2-zarf-packages/1-zarf-packages.md) page.
