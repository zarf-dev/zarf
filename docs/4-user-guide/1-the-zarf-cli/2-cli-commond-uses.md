# Common CLI Uses

Zarf optimizes the delivery of applications and capabilities into any environment, beginning with air-gapped systems. The Zarf CLI uses declarative Zarf Packages to create, deploy, inspect, and remove these applications and capabilities.

## Building Packages: `zarf package create`

[`zarf package create`](./100-cli-commands/zarf_package_create.md) is used to create a `tar.zst` archive that contains all the necessary dependencies and instructions to deploy capabilities onto another machine. We call this file a 'package'. The `package create` command looks for a `zarf.yaml` configuration file which describes all of the things that make up the package. It then uses the declarative definition of the package to perform all of the actions, such as downloading required container images and git repositories, to create the final package. More detailed information on Zarf packages can be found on the [Zarf](../2-zarf-packages/index.md) Packages](../2-zarf-packages/index.md) page.

## Initializing a Cluster: `zarf init`

<!-- TODO: Find a good place to talk about what the init command is actually doing (there's a lot of special magic sauce going on with that command) -->
<!-- TODO: Should we talk about the 'Zarf Agent - A Mutating Webhook' here? -->

Before you can deploy a package to a cluster, Zarf needs to initialize the cluster. This is done with [`zarf init`](./100-cli-commands/zarf_init.md). This command creates and bootstraps an in-cluster container registry. It also provides the ability to install optional tools and services into the cluster that future packages will need.

For Windows and macOS environments, A cluster needs to exist before Zarf can initialize it. You can use [Kind](https://kind.sigs.k8s.io/), [K3d](https://k3d.io/), [Docker Desktop](https://docs.docker.com/desktop/kubernetes/), or any other local or remote Kubernetes cluster.

For Linux, Zarf makes it even easier. If you don't have a cluster running, Zarf can take care of that for you. The init package used by `zarf init` also contains all the resources necessary to create a local [k3s](https://k3s.io/) cluster on your machine. The init package can be located in your current working directory, in the directory where the Zarf CLI binary lives, or be downloaded from the GitHub releases as the command is running. More information about the init package can be found on the [init package](../2-zarf-packages/3-the-zarf-init-package.md) page.

::: note
Depending on the permissions of your user, if you are installing k3s with `zarf init`, you may need to run it as a privileged user. This can be done by either:

1. Becoming a privileged user via the command `sudo su` and then running all the Zarf commands as you normally would.
2. Manually running all the Zarf commands as a privileged user via the command `sudo <command>`.
3. Running the init command as a privileged user via `sudo zarf init` and then changing the permissions of the `~/.kube/config` file to be readable by the current user.
   :::

## Deploying Packages: `zarf package deploy`

<!-- TODO: Write some docs (or reddirect to other docs) describing when you would be able to do a `zarf package deploy` before a `zarf init` -->

The [`zarf package deploy`](./100-cli-commands/zarf_package_deploy.md) command is where the air-gapped magic happens. It deploys our packaged capabilities into our target environment. It is usually assumed that the `zarf init` command has already been run on the machine you are deploying to but there are a few cases where this doesn't apply such as [YOLO Mode](../../9-faq.md#what-is-yolo-mode-and-why-would-i-use-it)

Since the package has all of its dependencies built-in, it can be deployed onto any cluster, even without an external internet connection. The dependency resources are pushed onto the cluster in their respective places, such as an in-cluster Gitea Git server or Docker registry, and then the application is deployed as instructed in the `zarf.yaml` file (i.e. deploying a helm chart, deploying raw k8s manifests, or even just executing a series of shell commands).

More information about Zarf packages is available on the [Understanding Zarf Packages](../2-zarf-packages/1-zarf-packages.md) page
