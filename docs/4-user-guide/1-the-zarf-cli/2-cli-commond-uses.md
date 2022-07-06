# Common CLI Uses

Since the main priority of Zarf is to make deploying applications into disconnected environments easier, almost all of the commands Zarf provides really boil down to making it easier to create and deploy packages. This is especially true with the most commonly used commands; `zarf init`, `zarf package create` and `zarf package deploy`. 


<br />


## Building Packages: `zarf package create`
[`zarf package create`](./cli-commands/package/zarf_package_create) is used to create a tar.zst binary file that contains all the necessary dependencies and instructions to deploy a set of functionality onto another machine. We call this tar.zst file a 'package'. The 'package create' command looks for a file called `zarf.yaml` which describes all of the things that make up the package and then does all the work necessary to prepare the output tar.zst package, such as downloading required container images and git repositories. More information on what these Zarf packages are can be found in the [Zarf Packages](../zarf-packages/zarf-packages) page.

<br />

## Initializing a Cluster: `zarf init`
<!-- TODO: Find a good place to talk about what the init command is actually doing (there's a lot of special magic sauce going on with that command) -->
<!-- TODO: Should we talk about the 'Zarf Agent - A Mutating Webhook' here? -->
Before you are able to deploy onto a cluster, Zarf needs to initialize the cluster to be ready. This is done through the [`zarf init`](./cli-commands/zarf_init) command, which injects and starts-up an in-cluster container registry, along with other optional tools and services into your cluster within the Zarf namespace that future packages will need. If you don't have a cluster yet, this command can help with that too! This command uses a specialized package called an 'init-package' that also contains all the resources necessary to create a local k3s cluster on your machine. This init-package can either be located in your current working directory, in the directory where the Zarf CLI binary lives, or be downloaded from the GitHub releases as the command is running. More information about what the 'init' command is doing will be provided soon but more information about the init-package can be found on the [init-package](../zarf-packages/the-zarf-init-package) page.

:::note
Depending on the permissions of your user, if you are installing k3s through the `zarf init` command you may need to run the command as a privileged user. This can be done by either:

1. Becoming a privileged user via the command `sudo su` and then running all the Zarf commands as you normally would.
2. Manually running all the Zarf commands as a privileged user via the command `sudo {ZARF_COMMAND_HERE}`.
3. Running the init command as a privileged user via `sudo zarf init` and then changing the permissions of the `~/.kube/config` file to be readable by the current user.
:::

<br />

## Deploying Packages: `zarf package deploy`

<!-- The most common use case (like 99.9% of the time) is deploying onto a k8s cluster.. but that doesn't HAVE to be the case.. How do I write the docs for this then? -->
<!-- TODO: Write some docs (or reddirect to other docs) describing when you would be able to do a `zarf package deploy` before a `zarf init` -->
As stated many times now, the entire purpose of Zarf is to make it easier to deploy applications onto air gapped environments. This is where that magic happens! [`zarf package deploy`](./cli-commands/package/zarf_package_deploy) is used to deploy an already built tar.zst package onto a machine, usually specifically into a k8s cluster. It is usually assumed that the `zarf init` command has already been run on the machine you are deploying to but there are a few rare cases where this doesn't apply. 

Since the package has all of its dependencies built-in, it can be deployed onto any cluster, even without an external internet connection. The dependency resources are pushed onto the cluster in their respective places, such as an in-cluster Gitea Git server or Docker registry, and then the application is deployed as instructed in the `zarf.yaml` file (i.e. deploying a helm chart, deploying raw k8s manifests, or even just executing a series of shell commands).

More information about Zarf packages is available on the [Understanding Zarf Packages](../zarf-packages/zarf-packages) page
