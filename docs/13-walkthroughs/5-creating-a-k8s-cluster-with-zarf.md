# Creating a K8s Cluster with Zarf

In this walkthrough, we will demonstrate how to use Zarf on a fresh Linux machine to deploy a [k3s](https://k3s.io/) cluster through Zarf's `k3s` component.

## System Requirements
-  `root` access on a Linux machine

:::info REQUIRES ROOT
The 'k3s' component requires root access (not just `sudo`!) when deploying as it will modify your host machine to install the cluster.
:::

## Prerequisites
- The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([`git clone` Instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
- Zarf binary installed on your $PATH: ([Install Instructions](../1-getting-started/index.md#installing-zarf))
- An init-package built/downloaded: ([init-package Build Instructions](./0-using-zarf-package-create.md)) or ([Download Location](https://github.com/defenseunicorns/zarf/releases))

## Walkthrough

1. Run the `zarf init` command as `root`.

```sh
# zarf init
```

2. Confirm Package Deployment: <br/>
- When prompted to deploy the package select `y` for Yes, then hit the `enter` key. <br/>

3. Confirm k3s Component Deployment: <br/>
- When prompted to deploy the k3s component select `y` for Yes, then hit the `enter` key.

<iframe src="/docs/walkthroughs/k3s_init.html" height="750px" width="100%"></iframe>

:::tip
You can automatically accept the k3s component and confirm the package using the `--components` and `--confirm` flags.

```sh
$ zarf init --components="k3s" --confirm
```
:::

### Validating the Deployment
After the `zarf init` command is done running, you should see a k3s cluster running and a few `zarf` pods in the Kubernetes cluster.

```sh
# zarf tools monitor
```
:::note
You can press `0` if you want to see all namespaces and CTRL-C to exit
:::

### Accessing the Cluster as a Normal User
By default, the k3s component will only automatically provide cluster access to the root user. To access the cluster as another user, you can run the following to setup the `~/.kube/config` file:

```sh
# cp /root/.kube/config /home/otheruser/.kube
# chown otheruser /home/otheruser/.kube/config
# chgrp otheruser /home/otheruser/.kube/config
```

## Cleaning Up

The [`zarf destroy`](../2-the-zarf-cli/100-cli-commands/zarf_destroy.md) command will remove all of the resources, including the k3s cluster, that was created by the initialization command.

```sh
zarf destroy --confirm
```
