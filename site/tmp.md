---
title: tmp DELETE ME
---

## Target Use Cases

- Make the delivery of software "across the air gap" an open-source "solved problem".
- Make it trivial to deploy and run Kubernetes apps "at the Edge".
- Make it easy to support GitOps-based K8s cluster updates in isolated environments.
- Make it possible to support GitOps-based K8s cluster updates in internet-connected-but-independent environments (think: dependency caching per availability zone, etc).

## How Zarf Works

Zarf simplifies and standardizes the delivery of complex software deployments. This gives users the ability to reduce tens/hundreds of individual software updates, data transfers, and manual installations to a few simple terminal commands. This tool equips users with the ability to pull, package, and install all of the resources their applications or clusters need to run without being connected to the internet. It can also deploy any necessary resources needed to stand up infrastructure tooling (such as Open Tofu / Terraform).

![Zarf CLI + Zarf Init + Zarf Package](../../assets/zarf-bubbles.svg)

A typical Zarf deployment is made up of three parts:

1. The [`zarf` binary](/cli/):
   - Is a statically compiled Go binary that can be run on any machine, server, or operating system with or without connectivity.
   - Creates packages combining numerous types of software/updates into a single distributable package (while on a network capable of accessing them).
   - Declaratively deploys package contents "into place" for use on production systems (while on an isolated network).
2. A [Zarf init package](/create-a-package/init-package/):
   - A compressed tarball package that contains the configuration needed to instantiate an environment without connectivity.
   - Automatically seeds your cluster with a container registry or wires up a pre-existing one
   - Provides additional capabilities such as logging, git server support, and/or a K8s cluster.
3. A [Zarf Package](/create-a-package/packages/):
   - A compressed tarball package that contains all of the files, manifests, source repositories, and images needed to deploy your infrastructure, application, and resources in a disconnected environment.

:::note

For more technical information on how Zarf works and to view the Zarf architecture, visit our [Nerd Notes page](./12-contribute-to-zarf/3-nerd-notes.md).

:::

## How To Use Zarf

Zarf is intended for use in a software deployment process that looks similar to this:

![How Zarf works](../../assets/what-is-zarf/how-to-use-zarf.drawio.png)

### (0) Connect to the Internet

Zarf doesn't build software—it helps you distribute software that already exists.

Zarf can pull from various places like Docker Hub, Iron Bank, GitHub, private registries and local filesystems. In order to do this, you must ensure that Zarf has a clear path and appropriate access credentials. Be sure you know what you want to pack and how to access it before you begin using Zarf.

### (1) Create a Package

This part of the process requires access to the internet (or a network that mirrors your resources). When the `zarf` binary is presented with a `zarf.yaml`, it then begins downloading, packing, and compressing the software that you requested. It then outputs a single, ready-to-move distributable called "a package".

For additional information, see the [Creating a package](./5-zarf-tutorials/0-creating-a-zarf-package.md) section.

### (2) Ship the Package to the System Location

Zarf enables secure software delivery for various environments, such as remote, constrained, standalone, and air-gapped systems. Considering there are various target environments with their own appropriate transferring mechanisms, Zarf does not determine _how_ packages are moved so long as they can arrive in your downstream environment.  See [Package Sources](./4-deploy-a-zarf-package/2-package-sources.md) for more information on where Zarf packages can be stored / pulled from.

### (3) Deploy the Package

Once your package has arrived, you will need to:

1. Install the binary onto the system.
2. Initialize a cluster with a zarf init package (`zarf init`)
3. Deploy the package to your cluster (`zarf package deploy`)

## Cluster Configuration Options

Zarf allows the package to either deploy to a K3s cluster it creates or an existing K8s cluster. This configuration is available on deployment of the init package.

### Initialize `k3s` as an Appliance

![Appliance Cluster Diagram](../../assets/what-is-zarf/appliance.drawio.png)

In the simplest usage scenario, you deploy the Zarf init package's builtin cluster and use it to serve your application(s) directly to end users. This configuration runs Zarf and it's init package components as a self-contained appliance and is intended for use in environments where you want to run K8s-native tooling but need to keep a small footprint (i.e. single-purpose/constrained/"Edge" environments).

### Initialize `k3s` as a Utility Cluster

![Utility Cluster Diagram](../../assets/what-is-zarf/utility-cluster.drawio.png)

In a more complex use case, you deploy the Zarf init package's builtin cluster and use it to serve resources to further downstream clusters. This configuration makes your Zarf deployment a utility cluster in service of a larger system and is intended for use in places where you want to run independent, full-service production environments with their own lifecycles but you want help tracking, caching and disseminating system/dependency updates.

### Skip `k3s` and Initialize to an Existing Cluster

![Existing Cluster Diagram](../../assets/what-is-zarf/existing-cluster.drawio.png)

In this use case, you configure Zarf to initialize a cluster that already exists within your environment, and use that existing cluster to host and serve your applications.  This configuration is intended for environments that may already have some supporting infrastructure such as disconnected / highly regulated cloud environments.
