# Zarf Packages

## What is a Zarf Package?

Zarf allows you to bundle portions of "the internet" into a single package to be installed later following specific instructions. A Zarf package is just a single tarball file that includes everything you need to manage a system or capability while fully disconnected. Think of a disconnected system as a system that always is or sometimes is on airplane mode.

## How does a Zarf Package work?

You bring this single file (we call it a package) to the system you want to install or update new software on. The package includes instructions on how to assemble all the pieces of software (components) once on the other side. These instructions are fully "declarative," which means that everything is represented by code and automated instead of manual. The hardest part is assembling the declarative package on the connected side. But once everything is packaged, Zarf makes even very complex systems easy to install, update, and maintain within disconnected systems.

## Using Zarf Packages

Such packages also become highly distributable, as they can now run on edge, embedded systems, secure cloud, data centers, or even in a local environment. This is incredibly helpful for organizations that need to integrate and deploy software from multiple, secure development environments from disparate development teams into disconnected IT operational environments. Zarf helps ensure that development teams can integrate with the production environment they are deploying to, even if they will never actually touch that environment.

## Dependencies for Deploying Zarf Packages

:::info

**Dependencies**

- A Zarf CLI ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../the-zarf-cli/building-your-own-cli))
- A Zarf init package ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../the-zarf-cli/building-your-own-cli))
- A Zarf Package (provided externally or [manually built](./zarf-packages#building-a-package))
- kube-context into a k8s cluster

  - (NOTE: Not needed if you plan on deploying the cluster with `zarf init` step)

:::
