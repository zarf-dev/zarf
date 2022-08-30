# Initializing a New K8s Cluster

:::caution Hard Hat Area
This page is still being developed. More content will be added soon!
:::

In this walkthrough, we are going to show how you can use Zarf on a fresh linux machine to deploy a [k3s](https://k3s.io/) cluster through Zarf's `k3s` component


## Walkthrough Prerequisites
1. The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([`git clone` Instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
1. Zarf binary installed on your $PATH: ([Install Instructions](../3-getting-started.md#installing-zarf))
1. An init-package built/downloaded: ([init-package Build Instructions](./0-creating-a-zarf-package.md)) or ([Download Location](https://github.com/defenseunicorns/zarf/releases))
1. kubectl: ([kubectl Install Instructions](https://kubernetes.io/docs/tasks/tools/#kubectl))
1. `root` access on a Linux machine

## Install the k3s component

To install the k3s component, follow the [Initializing a Cluster Instructions](./1-initializing-a-k8s-cluster.md) as `root`, and instead answer `y` when asked to install the `k3s` component
