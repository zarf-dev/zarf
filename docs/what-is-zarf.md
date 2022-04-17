# What is Zarf?

<img align="right" alt="zarf logo" src="../.images/zarf-logo.png" width="160" />

Zarf was created to _**support the declarative creation & distribution of software "packages" into remote / constrained / independent Kubernetes systems**_.

> "Zarf is a tool to help deploy modern stacks into air gapped environments; it's all about moving the bits." &mdash; Jeff

\* Check out our [term list](#what-we-mean-by) below for clarity on all those fancy words.

&nbsp;

## How it helps

Zarf supercharges the way you deliver complex, k8s-native applications to remote systems by reducing tens / hundreds of individual software updates, movements, and manual installations to a few simple terminal commands. Its support comes in two parts:

A precompiled `zarf` binary, which:

- rolls numerous software types / updates into a single distributable package (while on an internet-accessible network), and

- unrolls package contents "into place" for use on production systems (while on an internet-isolated network).

As well as a lightweight, long-running k8s cluster&mdash;called "**the Zarf cluster**"&mdash;which:

- hosts the services necessary for exposing package contents to your isolated, downstream consumers.

&nbsp;


## How it's used (from 30,000 ft)

Zarf is intended for use in a software deployment process that looks something like this:

<a href="../.images/what-is-zarf/how-to-use-it.png">
  <img alt="how it works" src="../.images/what-is-zarf/how-to-use-it.png" heigth="262" />
</a>

### (0) - The internet

Zarf wants to go out into the world to pull & package application software, so 1) the world has to exist, and 2) you need to make sure Zarf can reach the required resources within it (i.e. has a connection to the internet).

That should probably go without saying, but... it's always the "little things" that trip you up, right?

### (1) - Zarf builds you a package

You feed the `zarf` binary a "recipe" (`zarf.yaml`) and it makes itself busy downloading, packing, and compressing the software you asked for. It outputs a single, ready-to-move distributable (cleverly) called "a package".

Find out more about what that looks like in the "[Building a package](#building-a-package)" section.

### (2) - You ship the package

When it comes to remote / constrained / independent / air gapped systems, everyone's unique. Zarf doesn't have an opinion as to _how_ packages move just so long as you can get them into your downstream environment.

Find out more about what that _might_ look like in the "[Shipping a package](#shipping-a-package)" section.

### (3) - Zarf publishes your package

Once your package has landed, depending on what you've built into it, there are a couple of ways for the Zarf cluster to expose package contents:

In the simplest usage scenario, your package consists of a single application (plus dependencies) and you configure the Zarf cluster to _serve your application directly to end users_. This mode of operation is called "Appliance Mode"&mdash; because it's small & self-contained like a kitchen appliance&mdash;and it is intended for use in environments where you want to run k8s-native tooling but need to keep a small footprint (i.e. single-purpose / constrained / "edge" environments).

Find out more about what direct user service looks like in the "[Appliance Mode](#appliance-mode)" section.

In the standard, more complex usage scenario, your package consists of updates for many apps / systems and you configure the Zarf cluster to _propagate updates to downstream systems_ rather than to serve users directly. This mode of operation is called "Utility Mode"&mdash;as it's main job is to _add utility_ to other clusters&mdash;and it is intended for use in places where you want to run independent, full-service production environments (your own [Big Bang](https://github.com/DoD-Platform-One/big-bang) cluster, perhaps?) but you need help tracking, caching & disseminating system / dependency updates.

Find out more about what downstream cluster service looks like in the "[Utility Mode](#utility-mode)" section.

&nbsp;


## How it's used (from 1,000 ft)

### Building a package

<a href="../.images/what-is-zarf/build-the-package.png">
  <img alt="how it works" src="../.images/what-is-zarf/build-the-package.png" height="290" />
</a>

#### (0) - Identify software to-be-moved

Zarf doesn't build software&mdash;it helps you distribute software that already exists.

Zarf can pull from lots of places like [Docker Hub](https://hub.docker.com/), [Iron Bank](./ironbank.md), [GitHub](https://github.com/), local filesystems, etc. but you have to make sure that Zarf has a clear path & appropriate access credentials. Be sure you know what you want pack & how to access it before you Zarf.

Find out more about the types of software Zarf can move in the "[What can be packaged?](#what-can-be-packaged)" section.

#### (1) - Preparation

In order to build a Zarf package, you need to prepare:

- a "packaging" workstation &mdash; which must have the `zarf` [CLI tool installed](./workstation.md#just-gimmie-zarf), and

- a `zarf.yaml` file &mdash; which tells the Zarf what you need packaged.

Find some detailed uses of the `zarf.yaml` file in [our examples](../examples/).

#### (2) - Package

Actually making a Zarf package out of a `zarf.yaml` file is a matter of calling a single, simple command: `zarf package create`.  You'll see a `zarf-package-*.tar.zst` file pop into existence afterward&mdash;that's your package.

Find out more about that by calling the CLI for help, or check out an example package build in [our game example](../examples/game#package-the-game).

&nbsp;


### Shipping a package

<a href="../.images/what-is-zarf/ship-the-package.png">
  <img alt="how it works" src="../.images/what-is-zarf/ship-the-package.png" height="255" />
</a>

What this activity looks like is _very_ situationally specific so Zarf tries not to have an opinion&mdash;transfer Zarf packages between production & operating locations using whatever mechanisms are available.

Have to burn your package to disk & "sneakernet" it? That works.

Got an intermittent, super-secret satellite uplink you can use? Awesome.

Can you make a direct network connection? Even better.

Consider the art of the possible and use what you can&mdash;Zarf will continue to work, regardless.

&nbsp;


### Appliance Mode

<a href="../.images/what-is-zarf/appliance-mode.png">
  <img alt="how it works" src="../.images/what-is-zarf/appliance-mode.png" height="295" />
</a>

#### (0) - Package

Move a Zarf release + your desired packages to your Zarf cluster machine.

#### (1) - Zarf cluster

Make your machine into a **single node** Zarf cluster with the command: `zarf init`.

Recommended Zarf components: `k3s`.

Deploy your package into the Zarf cluster with the command: `zarf package deploy`.

#### (✓) - Use it

Connect directly to the Zarf cluster to access your newly unpackaged applications!

&nbsp;


### Utility Mode

<a href="../.images/what-is-zarf/utility-mode.png">
  <img alt="how it works" src="../.images/what-is-zarf/utility-mode.png" height="283" />
</a>

#### (0) - Package

Move a Zarf release + desired packages to your Zarf cluster machine.

#### (1) - Zarf cluster

Configure your system to talk to an existing Kubernetes cluster, then run `zarf init`

Recommended Zarf components: `git-server`.

Deploy your package into the Zarf cluster with the command: `zarf package deploy`.

> Utility Mode can be used in "single node K3s" mode too, just add the `k3s` component as well when running `zarf init`.

#### (2) - Downstream cluster

Point your downstream cluster to the Zarf cluster **source code** & **container image repository** for access to base software & updates (hint: [GitOps](https://www.weave.works/technologies/gitops/)).

#### (✓) - Use it

Once your downstream cluster has settled accounts with the Zarf cluster, connect to the downstream cluster to access your newly unpackaged applications!

&nbsp;


## Target Use Cases

Zarf's possibilities are endless&mdash;Zarf developers' time is not. Thus, scope definition is in order.

Here are the things we think Zarf should get really good at, listed top-down in order of importance:

- Make movement of software "across the air gap" an open source "solved problem".

- Make it trivial to deploy & run Kubernetes apps "at the Edge".

- Make it easy to support GitOps-based k8s cluster updates in isolated environments.

- Make it possible to support GitOps-based k8s cluster updates in internet-connected-but-independent environments (think: dependency caching per availability zone, etc).

&nbsp;


## What can be packaged?

To reiterate: Zarf's possibilities are endless&mdash;Zarf developers' time is not. Thus, scope definition is again in order.

Given Zarf's being a "k8s cluster to serve _other_ k8s clusters", the following types of software can be rolled into a Zarf package:

- container images &mdash; to serve images for the Zarf & downstream clusters to run containers from.

- git repositories &mdash; to serve as the git-based "source of truth" for downstream "GitOps"ed k8s clusters to watch.

- pre-compiled binaries &mdash; to provide the software necessary to start & support the Zarf cluster.

&nbsp;


## What we mean by...

**declarative** &mdash; A user states (via configuration file) which resources are needed and Zarf locates & packages them. A user does not have to know _how_ to download / collect / roll & unroll dependencies for transport, they only have to know _what_ they need.

**package** &mdash; A well-defined (tool-generated / versioned / compressed) collection of software intended for movement (and later use) across a network / adminstrative boundary.

**remote systems** &mdash; Systems organized such that development & maintenance actions occur _primarily_ in locations physically & logically separate from where operations occur.

**constrained systems** &mdash; Systems with explicit resource / adminstrative / capability limitations.

**independent systems** &mdash; Systems organized such that continued operation is possible even when disconnected (temporarily or otherwise) from external systems dependencies.

**air gapped systems** &mdash; Systems designed to operate while _physically disconnected_ from "unsecured" networks like the internet. More on that [here](https://en.wikipedia.org/wiki/Air_gap_(networking)).

&nbsp;
