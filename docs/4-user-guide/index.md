# User Guide

Experience just how easy it is to go from zero to chainsaw-wielding hero (of the Kubernetes cluster) using Zarf!

This guide is intended for end users who are using Zarf in a disconnected environment, and contains information on how to use and configure Zarf's major features:

- Deploy Zarf [Packages](2-zarf-packages/1-zarf-packages.md) (Zpkg)
- Maintain Zarf Packages in the cluster
- A reference of all [CLI commands](1-the-zarf-cli/100-cli-commands/zarf.md)
- Autogenerate and view a package SBOM
- Add logging to your cluster
- A list of [supported Zarf packages](2-zarf-packages/1-zarf-packages.md)

## Overview of Zarf Workflow

### Create a package

<a target="\_blank" href={require('../.images/what-is-zarf/build-the-package.png').default}>
  <img alt="diagram showing the process to build a package" src={require('../.images/what-is-zarf/build-the-package.png').default} height="290" />
</a>

#### (0) - Identify software to-be-moved

Zarf doesn't build software; it helps you distribute software that already exists.

Zarf can pull from sources like [Docker Hub](https://hub.docker.com/), [Iron Bank](https://p1.dso.mil/products/iron-bank), [GitHub](https://github.com/), local filesystems, etc. but you have to make sure that Zarf has a clear path & appropriate access credentials. Be sure you know what you want to pack & how to access it before you Zarf.

Find out more about the types of software Zarf can move in the "[What can be packaged?](../0-zarf-overview.md#what-can-be-packaged#what-can-be-packaged)" section.

#### (1) - Preparation

To build a Zarf package, you need to prepare:

- a "packaging" workstation which must have the `zarf` [CLI tool installed](../3-getting-started.md#installing-zarf), and

- a `zarf.yaml` file which tells Zarf what you need to package.

Find some detailed uses of the `zarf.yaml` file in [our examples](../../examples/).

#### (2) - Package

Making a Zarf package out of a `zarf.yaml` file is a matter of calling a single, simple command: `zarf package create`. You'll see a `zarf-package-*.tar.zst` file pop into existence afterward. That's your package.

Find out more about that by calling the CLI for help, or check out an example package build in [our game example](../../examples/game#package-the-game).

&nbsp;

### Ship Package

<a target="\_blank" href={require('../.images/what-is-zarf/ship-the-package.png').default}>
  <img alt="diagram showing the process to ship a package" src={require('../.images/what-is-zarf/ship-the-package.png').default} height="255" />
</a>

What this activity looks like is _very_ contextual to the target environment, so Zarf tries not to have an opinion. Transfer Zarf packages between production & operating locations using whatever mechanisms are appropriate.

Have to burn your package to disk & "sneakernet" it? That works.

Got an intermittent, super-secret satellite uplink you can use? Awesome.

Can you make a direct network connection? Even better.

Consider the art of the possible and use what you can. Zarf will work, regardless.

### Deploy Package

&nbsp;

## Other Resources

If you are looking for more advanced information on how to operate and customize Zarf to your specific environment needs, check out these additional resources.

- For information on how to create a custom configuration of the Zarf CLI see the [Operator Manual](../5-operator-manual/_category_.json)
- For information on how to create your own Zarf Packages see the [Developer Guide](../6-developer-guide/1-contributor-guide.md)
- To see some of the ways our community is using Zarf to deploy code onto AirGap systems see the Zarf [Examples](../../examples/README.md)
