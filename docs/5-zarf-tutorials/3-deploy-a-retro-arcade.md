# Deploying a Retro Arcade

## Introduction

In previous tutorials, we learned how to [create a package](./0-creating-a-zarf-package.md), [initialize a cluster](./1-initializing-a-k8s-cluster.md), and [deploy a package](./2-deploying-zarf-packages.md). In this tutorial, we will leverage all that past work and deploy a fun application onto your cluster.

## System Requirements

- You'll need an internet connection to grab the Zarf Package for the games example.

## Prerequisites

Before beginning this tutorial you will need the following:

- The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([git clone instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
- Zarf binary installed on your $PATH: ([Installing Zarf](../1-getting-started/index.md#installing-zarf))
- [An initialized cluster](./1-initializing-a-k8s-cluster.md)

## YouTube Tutorial
[![Deploying Packages with Zarf Video on YouTube](../.images/tutorials/package_deploy_thumbnail.jpg)](https://youtu.be/7hDK4ew_bTo "Deploying Packages with Zarf")

## Deploying the Arcade

1. The `dos-games` package is easily deployable via `oci://` by running `zarf package deploy oci://ghcr.io/defenseunicorns/packages/dos-games:1.0.0-$(uname -m) --key=https://zarf.dev/cosign.pub`.

:::tip

You can publish your own packages for deployment too via `oci://`.  See the [Store and Deploy Packages with OCI](./7-publish-and-deploy.md) tutorial for more information.

:::

<iframe src="/docs/tutorials/package_deploy_deploy.html" width="100%" height="595px"></iframe>

2. If you did not use the `--confirm` flag to automatically confirm that you want to deploy this package, press `y` for yes.  Then hit the `enter` key.

<iframe src="/docs/tutorials/package_deploy_deploy_bottom.html" width="100%" height="400px"></iframe>

### Connecting to the Games

When the games package finishes deploying, you should get an output that lists a couple of new commands that you can use to connect to the games. These new commands were defined by the creators of the games package to make it easier to access the games. By typing the new command, your browser should automatically open up and connect to the application we just deployed into the cluster, using the `zarf connect` command.

<iframe src="/docs/tutorials/package_deploy_connect.html" width="100%"></iframe>

![Connected to the Games](../.images/tutorials/games_connected.png)

:::note
If your browser doesn't automatically open up, you can manually go to your browser and copy the IP address that the command printed out into the URL bar.
:::

:::note
The `zarf connect games` will continue running in the background until you close the connection by pressing the `ctrl + c` (`control + c` on a mac) in your terminal to terminate the process.
:::

## Removal

1. Use the `zarf package list` command to get a list of the installed packages.  This will give you the name of the games package to remove it.

<iframe src="/docs/tutorials/package_deploy_list.html" height="120px" width="100%"></iframe>

2. Use the `zarf package remove` command to remove the `dos-games` package.  Don't forget the `--confirm` flag.  Otherwise you'll receive an error.

<iframe src="/docs/tutorials/package_deploy_remove_no_confirm.html" width="100%" height="425px"></iframe>

3. You can also use the `zarf package remove` command with the zarf package file, to remove the package.  Again don't forget the `--confirm` flag.

<iframe src="/docs/tutorials/package_deploy_remove_by_file.html" height="100px" width="100%"></iframe>

The dos-games package has now been removed from your cluster.

## Troubleshooting

### Unable to connect to the Kubernetes cluster

<iframe src="/docs/tutorials/troubleshoot_unreachable.html" width="100%" height="200px"></iframe>

:::info Remediation

If you receive this error, either you don't have a Kubernetes cluster, your cluster is down, or your cluster is unreachable.

1. Check your kubectl configuration, then try again.  For more information about kubectl configuration see [Configure Access to Multiple Clusters](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) from the Kubernetes documentation.

If you need to setup a cluster, you can perform the following.

1. Deploy a Kubernetes cluster with the [Creating a K8s Cluster with Zarf](./5-creating-a-k8s-cluster-with-zarf.md) tutorial.
2. Perform the [Initialize a cluster](./1-initializing-a-k8s-cluster.md) tutorial.

After that you can try deploying the package again.

:::

### Secrets "zarf-state" not found

<iframe src="/docs/tutorials/troubleshoot_uninitialized.html" width="100%" height="250px"></iframe>

:::info Remediation

If you receive this error when zarf is attempting to deploy the `BASELINE COMPONENT`, this means you have not initialized the kubernetes cluster.  This is one of the prerequisites for this tutorial.  Perform the [Initialize a cluster](./1-initializing-a-k8s-cluster.md) tutorial, then try again.

:::

## Credits

:sparkles: Special thanks to these fine references! :sparkles:

- <https://www.reddit.com/r/programming/comments/nap4pt/dos_gaming_in_docker/>
- <https://earthly.dev/blog/dos-gaming-in-docker/>
