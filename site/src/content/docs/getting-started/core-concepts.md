---
title: Core Concepts
---

Now, assuming you're familiar with Kubernetes, AirGap, and GitOps from [Understanding the Basics](basics), we can get started on the core concepts of Zarf.

## Deployments

![Zarf CLI + Zarf Init + Zarf Package](../../../assets/zarf-bubbles.svg)

A typical Zarf deployment is made up of three parts:

1. The [`zarf` binary](../cli/index.mdx):
   - Is a statically compiled Go binary that can be run on any machine, server, or operating system with or without connectivity.
   - Creates packages combining numerous types of software/updates into a single distributable package (while on a network capable of accessing them).
   - Declaratively deploys package contents "into place" for use on production systems (while on an isolated network).
2. A [Zarf init package](../create-a-package/init-package.mdx):
   - A compressed tarball package that contains the configuration needed to instantiate an environment without connectivity.
   - Automatically seeds your cluster with a container registry or wires up a pre-existing one
   - Provides additional capabilities such as logging, git server support, and/or a K8s cluster.
3. A [Zarf Package](../create-a-package/packages.mdx):
   - A compressed tarball package that contains all of the files, manifests, source repositories, and images needed to deploy your infrastructure, application, and resources in a disconnected environment.

## Zarf Concepts

{/* - [**Zarf Package**](../3-create-a-zarf-package/1-zarf-packages.md) - A binary file that contains the instructions and dependencies necessary to install an application on a system.
- [**Zarf Component**](../3-create-a-zarf-package/2-zarf-components.md) - A set of defined functionality and resources that build up a package.
- [**Zarf Init Package**](../3-create-a-zarf-package/3-zarf-init-package.md) - The initial package that lays the groundwork for other packages. */}
