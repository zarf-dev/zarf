---
sidebar_position: 3
---

# The Zarf 'init' Package

The init package is the `zarf.yaml` file that lives at the [root of the Zarf repository](https://github.com/defenseunicorns/zarf/blob/main/zarf.yaml).
It is defined by composed components which provide a foundation for future packages to utilize. Upon deployment, the init package generates a `zarf` namespace within your K8s cluster and deploys pods, services, and secrets to that namespace based on the optional components selected for deployment.

## Required Component

Zarf's capabilities require that the [`zarf-agent`](https://docs.zarf.dev/docs/faq#what-is-the-zarf-agent) component of the init package is constantly active, meaning that it cannot be disabled and is always on. This component is automatically deployed whenever a `zarf init` command is executed.

| Component              | Description                                                                                                          |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------- |
| zarf-agent           | A Kubernetes mutating webhook installed during `zarf init` that convernts PodSpecs and Flux GitRepository objects to match their air gap equivalents.

## Core Components

In addition to the required `zarf-agent` component, Zarf also offers optional components that provide additional functionality and can be enabled as needed based on the state of your registry. 

In most scenarios, Zarf will also deploy an internal registry using the components described below. However, Zarf can be configured to use an already existing registry with the `--registry-*` flags when running `zarf init` (detailed information on all `zarf init` command flags can be found in the [zarf init CLI](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/cli-commands/zarf_init) section). This option skips the injector and seed process, and will not deploy a registry to the cluster. Instead, it uploads any images to the externally configured registry.

Additionally, below are the fully-optional components available for the init package, along with their respective component names that can be passed to `zarf init --components` to deploy them in an internal registry:

| Components   | Description                                                                                                                                                       |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| k3s          | REQUIRES ROOT. Installs a lightweight Kubernetes Cluster on the local host&mdash;[K3s](https://k3s.io/)&mdash;and configures it to start up on boot.                             |
| logging      | Adds a log monitoring stack&mdash;[promtail/loki/graphana (aka PLG)](https://github.com/grafana/loki)&mdash;into the cluster.                              |
| git-server   | Adds a [GitOps](https://www.cloudbees.com/gitops/what-is-gitops)-compatible source control service&mdash;[Gitea](https://gitea.io/en-us/)&mdash;into the cluster. |
| zarf-injector           | Adds a Rust and Go binary to the working directory to use during the registry bootstrapping.
| container-registry-seed | Adds a container registry so Zarf can bootstrap itself into the cluster.                                             |
| container-registry      | Adds a container registry service&mdash;[docker registry](https://docs.docker.com/registry/)&mdash;into the cluster. |

There are two ways to deploy optional components. Firstly, you can provide a comma-separated list of components to the `--components` flag, such as `zarf init --components k3s,git-server --confirm`. Alternatively, you can choose to exclude the `--components` and `--confirm` flags and respond with a yes or no for each optional component when prompted.

:::note

Deploying the 'k3s' component will require root access (not just sudo), as it modifies your host machine to install the cluster.
 
:::

## What Makes the Init Package Special

Deploying onto air-gapped environments is a [hard problem](../../1-understand-the-basics.md#what-is-the-air-gap), particularly when the K8s environment doesn't have a container registry for you to store images. This results in a dilemma where the container registry image must be introduced to the cluster, but there is no container registry to push it to as the image is not yet in the cluster. To ensure that our approach is distro-agnostic, we developed a unique solution to seed the container registry into the cluster.

To address this problem, we use the `zarf-injector` [component](https://github.com/defenseunicorns/zarf/blob/main/packages/zarf-injector/zarf.yaml) within the init-package. This resolves the issue by injecting a single rust binary (statically compiled) and a series of configmap chunks of a `registry:2` image into a ephemeral pod that is based on an existing image in the cluster.
