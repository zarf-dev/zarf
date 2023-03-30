# User Guide

Using Zarf optimizes the delivery of applications and capabilities in air-gapped and complex environments. This tool eliminates the complexity of air gap software delivery for Kubernetes clusters and cloud-native workloads using a declarative packaging strategy to support DevSecOps. This guide is intended for end users using Zarf to securely and efficiently deploy modern stacks onto remote/constrained/independent environments.

The below list contains information on how to use and configure Zarf’s major features: 

- Deploy Zarf [Packages](2-zarf-packages/1-zarf-packages.md) (Zpkg).
- Maintain Zarf Packages in the cluster.
- A reference of all [CLI commands](1-the-zarf-cli/100-cli-commands/zarf.md).
- Autogenerate and view a package SBOM.
- Add logging to your cluster.
- A list of [supported Zarf Packages](2-zarf-packages/1-zarf-packages.md).

## Overview of Zarf Workflow

### Create a Package

<a target="\_blank" href={require('../.images/what-is-zarf/build-the-package.png').default}>
  <img alt="diagram showing the process to build a package" src={require('../.images/what-is-zarf/build-the-package.png').default} height="290" />
</a>

#### (0) Identify Software to be Packaged

Zarf doesn't build software—it helps you distribute software that already exists.

Zarf can pull from sources like [Docker Hub](https://hub.docker.com/), [Iron Bank](https://p1.dso.mil/products/iron-bank), [GitHub](https://github.com/), and local filesystems. In order to do this, you must ensure that Zarf has a clear path and appropriate access credentials. Be sure you know what you want to pack and how to access it before you begin using Zarf.

:::note

Find out more about the types of software Zarf can move in the [What can be Packaged](../0-zarf-overview.md#what-can-be-packaged) section.

:::

#### (1) Preparation

To build a Zarf package, you will need to prepare:

- A "packaging" workstation which must have the `zarf` [CLI tool installed](../3-getting-started.md#installing-zarf).

- A `zarf.yaml` file which tells Zarf what you need to package.

:::note

For additional information and detailed uses of the `zarf.yaml` file, see [our examples](../../examples/) page.

:::

#### (2) Package

Making a Zarf Package out of a `zarf.yaml` file is a matter of calling a single command: `zarf package create`. You will see a `zarf-package-*.tar.zst` file populate aftwerards—that's your package.

:::note

For more information you can call the CLI for help, or check out an example package build in [our game example](../../examples/dos-games#package-the-game).

:::

### Ship Package

<a target="\_blank" href={require('../.images/what-is-zarf/ship-the-package.png').default}>
  <img alt="diagram showing the process to ship a package" src={require('../.images/what-is-zarf/ship-the-package.png').default} height="255" />
</a>

Shipping a Zarf Package is _very_ contextual to the target environment. Considering there are various target environments with their own appropriate transferring mechanisms, Zarf does not determine _how_ packages are moved so long as they can arrive in your downstream environment. Transfer Zarf Packages between production and operating locations using whatever mechanisms are appropriate for your mission.

There are numerous methods to transport your Zarf Package, for example:

- Burning your package onto a disk. 
- Using a satellite uplink.
- Creating a direct internet connection.

No matter the system complexity or internet connectivity, Zarf will work regardless.

### Deploy Package

Once your package has arrived, you will need to:

1. Install the binary onto the system.
2. Run the zarf init package.
3. Deploy the package to your cluster.

## Additional Resources

If you are looking for more advanced information on how to operate and customize Zarf to your specific environment needs, check out these additional resources:

- For information on how to create a custom configuration of the Zarf CLI see the [Operator Manual](../5-operator-manual/_category_.json).
- For information on how to create your own Zarf Packages see the [Developer Guide](../6-developer-guide/1-contributor-guide.md).
- To see some of the ways our community is using Zarf to deploy code onto air-gapped systems see the [Zarf Examples](../../examples/README.md).
