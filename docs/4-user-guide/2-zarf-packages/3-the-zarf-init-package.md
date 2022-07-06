---
sidebar_position: 3
---

# The Zarf 'init' Package

The init package is the zarf.yaml file that lives at the [root of the Zarf repository](https://github.com/defenseunicorns/zarf/blob/master/zarf.yaml). It is defined via composed components that all offer value for future packages to utilize. When the init package is deployed, it will create a `zarf` namespace within your k8s cluster and deploy various pods, services, and secrets to that namespace, depending on which optional components you choose to deploy.


## Mandatory Components

Zarf's work necessitates that some components in the [init package](https://github.com/defenseunicorns/zarf/blob/master/zarf.yaml) are "always on" (a.k.a. required & cannot be disabled). These components are always deployed whenever you perform a `zarf init` command. Those include:

|                         | Description                                                                                                          |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------- |
| zarf-injector           | Adds a Rust and Go binary to the working directory to use during the registry bootstrapping.
| container-registry-seed | Adds a container registry so Zarf can bootstrap itself into the cluster.                                             |
| container-registry      | Adds a container registry service&mdash;[docker registry](https://docs.docker.com/registry/)&mdash;into the cluster. |


&nbsp;
## Additional Components

In addition to those that are always installed, Zarf's optional components provide additional functionality and can be enabled as & when you need them.

These optional components for the init package are listed below along with the "magic strings" you pass to `zarf init --components` to pull them in:

| --components | Description                                                                                                                                                       |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| k3s          | REQUIRES ROOT. Installs a lightweight Kubernetes Cluster on the local host&mdash;[k3s](https://k3s.io/)&mdash;and configures it to start up on boot.                             |
| logging      | Adds a log monitoring stack&mdash;[promtail / loki / graphana (a.k.a. PLG)](https://github.com/grafana/loki)&mdash;into the cluster.                              |
| git-server   | Adds a [GitOps](https://www.cloudbees.com/gitops/what-is-gitops)-compatible source control service&mdash;[Gitea](https://gitea.io/en-us/)&mdash;into the cluster. |

There are two ways to deploy optional components, you can either pass a comma separated list of components to the `--components` flag such as `zarf init --components k3s,git-server --confirm` or you can exclude the flags and say yes/no as each optional component gets prompted to you.

> Note: The 'k3s' component requires root access when deploying as it will modify your host machine to install the cluster.

<br />

# What Makes the Init Package Special

Deploying onto air-gapped environments is a [hard problem](../../understand-the-basics#what-is-the-air-gap), especially when the k8s environment you're deploying to doesn't have a container registry running for you to put your images into. This leads to a classic 'chicken or the egg' problem since the container registry image needs to make its way into the cluster but there is on container registry running on the cluster to push to yet because the image isn't in the cluster yet. In order to remain distro agnostic, we had to come up with a unique solution to seed the container registry into the cluster.

The `zarf-injector` [component](https://github.com/defenseunicorns/zarf/blob/master/packages/zarf-injector/zarf.yaml) within the init-package solves this problem by injecting a really small [Go registry binary](https://github.com/defenseunicorns/zarf/blob/master/src/injector/stage2/registry.go) into the cluster by splitting the binary into small enough chunks that would fit inside of a k8s ConfigMap. Once th config map is pushed onto the cluster, it gets stitched back together and runs to bootstrap the registry.

<!-- TODO: Fix this link.. -->
More details about how we solved that problem is described in the [Seeding the Zarf Registry page](https://google.com).