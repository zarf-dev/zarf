# Deploying a Retro Arcade 

## Introduction

In previous walkthroughs, we learned how to [create a package](./0-using-zarf-package-create.md), [initialize a cluster](./1-initializing-a-k8s-cluster.md), and [deploy a package](./2-deploying-zarf-packages.md). In this walkthrough, we will leverage all that past work and deploy a fun application onto your cluster. While this example game is nothing crazy, this walkthrough hopes to show how simple it is to build packages and deploy them into a Kubernetes cluster.

## System Requirements

- You'll need an internet connection to grab the Zarf source code that includes the games example.

## Prerequisites

1. The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([git clone instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
2.  Zarf binary installed on your $PATH: ([Installing Zarf](../3-getting-started.md#installing-zarf))
3. [An initialized cluster](./1-initializing-a-k8s-cluster.md)

## YouTube Walkthrough
[![Deploying Packages with Zarf Video on YouTube](../.images/walkthroughs/package_deploy_thumbnail.jpg)](https://youtu.be/7hDK4ew_bTo "Deploying Packages with Zarf")

1. Navigate to the `dos-games` folder within the Zarf repo.

```sh
$ cd src/github.com/defenseunicorns/zarf/examples/dos-games
```

2. Use the `zarf package create .` command to create the Zarf games package. Enter `y` to confirm package creation.

<iframe src="/docs/walkthroughs/dos_games_create.html" width="100%" height="275px"></iframe>

3. Provide a file size for the package, or enter `0` to disable the feature.

<iframe src="/docs/walkthroughs/dos_games_size.html" width="100%" height="100px"></iframe>

Once you enter your response for the package size, the output that follows will show the package being created.

<iframe src="/docs/walkthroughs/dos_games_components.html" width="100%" height="300px"></iframe>

4. Use the `zarf package deploy` command to deploy the Zarf games package.

<iframe src="/docs/walkthroughs/package_deploy_deploy.html" width="100%" height="595px"></iframe>

5. If you do not provide the path to the package as an argument to the `zarf package deploy` command, Zarf will prompt you to choose which package you want to deploy.

<iframe src="/docs/walkthroughs/package_deploy_suggest.html" width="100%" height="150px"></iframe>

You can list all packages in the current directory by hitting `tab`. Then, use the arrow keys to select which package you want to deploy. If there is only one package available, hitting `tab` will autofill that one option. Since we are deploying the games package in this walkthrough, we will select that package and hit `enter`.

<iframe src="/docs/walkthroughs/package_deploy_suggestions.html" width="100%" height="150px"></iframe>
As we have seen a few times now, we are going to be prompted to confirm that we want to deploy this package onto our cluster.

<iframe src="/docs/walkthroughs/package_deploy_deploy.html" width="100%" height="595px"></iframe>


6. If you did not use the `--confirm` flag to automatically confirm that you want to deploy this package, press `y` for yes.  Then hit the `enter` key.

<iframe src="/docs/walkthroughs/package_deploy_deploy_bottom.html" width="100%" height="400px"></iframe>

### Connecting to the Games

When the games package finishes deploying, you should get an output that lists a couple of new commands that you can use to connect to the games. These new commands were defined by the creators of the games package to make it easier to access the games. By typing the new command, your browser should automatically open up and connect to the application we just deployed into the cluster, using the `zarf connect` command.

<iframe src="/docs/walkthroughs/package_deploy_connect.html" width="100%"></iframe>

![Connected to the Games](../.images/walkthroughs/games_connected.png)

:::note
If your browser doesn't automatically open up, you can manually go to your browser and copy the IP address that the command printed out into the URL bar.
:::

:::note
The `zarf connect games` will continue running in the background until you close the connection by pressing the `ctrl + c` (`control + c` on a mac) in your terminal to terminate the process.
:::

## Removal

1. Use the `zarf package list` command to get a list of the installed packages.  This will give you the name of the games package to remove it.

<iframe src="/docs/walkthroughs/package_deploy_list.html" width="100%"></iframe>

2. Use the `zarf package remove` command to remove the `dos-games` package.  Don't forget the `--confirm` flag.  Otherwise you'll receive an error.

<iframe src="/docs/walkthroughs/package_deploy_remove_no_confirm.html" width="100%" height="425px"></iframe>

3. You can also use the `zarf package remove` command with the zarf package file, to remove the package.  Again don't forget the `--confirm` flag.

<iframe src="/docs/walkthroughs/package_deploy_remove_by_file.html" width="100%"></iframe>

The dos-games package has now been removed from your cluster.

## Troubleshooting

### Unable to connect to the Kubernetes cluster

#### Example

<iframe src="/docs/walkthroughs/troubleshoot_unreachable.html" width="100%" height="200px"></iframe>

#### Remediation

If you receive this error, either you don't have a Kubernetes cluster, your cluster is down, or your cluster is unreachable.

1. Check your kubectl configuration, then try again.  For more information about kubectl configuration see [Configure Access to Multiple Clusters](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) from the Kubernetes documentation.

If you need to setup a cluster, you can perform the following.

1. Deploy a Kubernetes cluster with the [Creating a K8s Cluster with Zarf](./5-creating-a-k8s-cluster-with-zarf.md) walkthrough.
2. Perform the [Initialize a cluster](./1-initializing-a-k8s-cluster.md) walkthrough.

After that you can try deploying the package again.

### Secrets "zarf-state" not found

#### Example

<iframe src="/docs/walkthroughs/troubleshoot_uninitialized.html" width="100%" height="250px"></iframe>

#### Remediation

If you receive this error when zarf is attempting to deploy the `BASELINE COMPONENT`, this means you have not initialized the kubernetes cluster.  This is one of the prerequisites for this walkthrough.  Perform the [Initialize a cluster](./1-initializing-a-k8s-cluster.md) walkthrough, then try again.

## Credits

:sparkles: Special thanks to these fine references! :sparkles:

- <https://www.reddit.com/r/programming/comments/nap4pt/dos_gaming_in_docker/>
- <https://earthly.dev/blog/dos-gaming-in-docker/>
