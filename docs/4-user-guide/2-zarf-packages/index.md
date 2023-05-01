# Zarf Packages

## What is a Zarf Package?

Zarf enables you to consolidate portions of the internet into a single package that can be conveniently installed at a later time by following specific instructions. A Zarf Package is a single tarball file that includes all of the essential elements required for efficiently managing a system or capability, even when entirely disconnected from the internet. In this context, a disconnected system refers to a system that either consistently operates in an offline mode or occasionally disconnects from the network.

## How Does a Zarf Package Work?

Zarf facilitates the installation or updating of software on a desired system through a single file, referred to as a package. This package contains comprehensive instructions on assembling various software components once deployed on the targeted system. The instructions are fully "declarative", meaning that all components are represented by code and automated, eliminating the need for manual intervention. Zarf significantly streamlines the process of installing, updating, and maintaining even the most intricate systems in disconnected environments.

## Using Zarf Packages

Zarf Packages are highly distributable, allowing for seamless operation in diverse environments, such as edge systems, embedded systems, secure cloud, data centers, or even local environments. This is especially beneficial for organizations requiring the integration and deployment of software from multiple, secure development environments across development teams into disconnected IT operational environments. With Zarf, development teams can seamlessly integrate with the production environment they are deploying to, even if they have no direct contact with said environment.

## Dependencies for Deploying Zarf Packages

The following is a list of dependencies necessary for deploying Zarf Packages:

- A Zarf CLI ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../../2-the-zarf-cli/0-building-your-own-cli.md)).
- A Zarf init package ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../../2-the-zarf-cli/0-building-your-own-cli.md)).
- A Zarf Package (provided externally or [manually built](./1-zarf-packages.md#building-a-package)).
- kube-context into a K8s cluster.
  - (Not needed if you plan on deploying the cluster with `zarf init` step).
