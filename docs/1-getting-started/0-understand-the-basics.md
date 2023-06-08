# Understand the Basics

Before you can effectively use Zarf, it is useful to have an understanding of the technology Zarf is built on and around. The sections below provide some helpful links to start building up this foundation as well as a glossary of the terms used in this project.

<!-- TODO: The links and information on this page need to be expanded a lot more to be useful -->

## Technologies

### Kubernetes

- [What is Kubernetes?](https://www.ibm.com/cloud/learn/kubernetes)
- [Learn Kubernetes Basics](https://kubernetes.io/docs/tutorials/kubernetes-basics/)
- [Free Introduction to Kubernetes Course](https://www.edx.org/course/introduction-to-kubernetes)
- [Super charge your Kubernetes deployments](https://www.youtube.com/watch?v=N6UCKF7JD7k)

### AirGap Basics

- [What is AirGap](https://ibm.github.io/kubernetes-networking/vpc/airgap/)
- AirGap Kubernetes Course - Coming Soon!

### GitOps Basics

- [CloudBees GitOps Definition](https://www.cloudbees.com/gitops/what-is-gitops)
- [Understanding Git](https://hackernoon.com/understanding-git-fcffd87c15a3)

### CLI Basics

- [List of CLI Commands](https://www.codecademy.com/article/command-line-commands)
- [How to use the Command Line](https://training.linuxfoundation.org/training/linux-tools-for-software-development-lfd108x/)
- [Free Linux tools for Software Development Course](https://training.linuxfoundation.org/training/linux-tools-for-software-development-lfd108x/)

## Terms Used

- **Declarative**:  A user states (via configuration file) which resources are needed and Zarf locates and packages them. A user does not have to know _how_ to download, collect, roll, and unroll dependencies for transport, they only have to know _what_ they need.
- **Package**:  A well-defined, tool-generated, versioned, and compressed collection of software intended for movement (and later use) across a network/administrative boundary.
- **Remote systems**:  Systems that are organized such that development and maintenance actions occur _primarily_ in locations physically and logically separate from where operations occur.
- **Constrained systems**:  Systems with explicit resource/administrative/capability limitations.
- **Independent systems**:  Systems are organized such that continued operation is possible even when disconnected (temporarily or otherwise) from external systems dependencies.
- **Air-gapped systems**:  Systems are designed to operate while _physically disconnected_ from "unsecured" networks like the internet. For more information, see [Air Gap Networking](<https://en.wikipedia.org/wiki/Air_gap_(networking)>).
