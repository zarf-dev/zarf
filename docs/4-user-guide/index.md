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

<a href="../.images/what-is-zarf/build-the-package.png">
  <img alt="how it works" src="../.images/what-is-zarf/build-the-package.png" height="290" />
</a>

#### (0) - Identify software to-be-moved

Zarf doesn't build software&mdash;it helps you distribute software that already exists.

Zarf can pull from lots of places like [Docker Hub](https://hub.docker.com/), [Iron Bank](https://p1.dso.mil/products/iron-bank), [GitHub](https://github.com/), local filesystems, etc. but you have to make sure that Zarf has a clear path & appropriate access credentials. Be sure you know what you want pack & how to access it before you Zarf.

Find out more about the types of software Zarf can move in the "[What can be packaged?](#what-can-be-packaged)" section.

#### (1) - Preparation

In order to build a Zarf package, you need to prepare:

- a "packaging" workstation &mdash; which must have the `zarf` [CLI tool installed](../3-getting-started.md#installing-zarf), and

- a `zarf.yaml` file &mdash; which tells the Zarf what you need packaged.

Find some detailed uses of the `zarf.yaml` file in [our examples](../../examples/).

#### (2) - Package

Actually making a Zarf package out of a `zarf.yaml` file is a matter of calling a single, simple command: `zarf package create`. You'll see a `zarf-package-*.tar.zst` file pop into existence afterward&mdash;that's your package.

Find out more about that by calling the CLI for help, or check out an example package build in [our game example](../../examples/game#package-the-game).

&nbsp;

### Ship Package

<a href="../.images/what-is-zarf/ship-the-package.png">
  <img alt="how it works" src="../.images/what-is-zarf/ship-the-package.png" height="255" />
</a>

What this activity looks like is _very_ situationally specific so Zarf tries not to have an opinion&mdash;transfer Zarf packages between production & operating locations using whatever mechanisms are available.

Have to burn your package to disk & "sneakernet" it? That works.

Got an intermittent, super-secret satellite uplink you can use? Awesome.

Can you make a direct network connection? Even better.

Consider the art of the possible and use what you can&mdash;Zarf will continue to work, regardless.

### Deploy Package

&nbsp;

## Other Resources

If you are looking for more advanced information on how to operate and custom configure Zarf to your specific environment needs, check out these additional resources.

- For information on how to create custom configuration of the Zarf CLI see the [Operator Manual](../5-operator-manual/_category_.json)
- For information on how to create your own custom Zarf Packages see the [Developer Guide](../6-developer-guide/1-contributor-guide.md)
- To see some of the ways our community is using Zarf to deploy code onto AirGap systems see the Zarf [Examples](../../examples/README.md)
