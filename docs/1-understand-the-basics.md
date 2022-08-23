# Understand The Basics

Before you are able to effectively use Zarf, it is would be useful to have an underlying understanding of the technology Zarf is built on/around. The section below provides some helpful links to start build up this foundation as well as a glossary of the terms used in this project.

:::caution Hard Hat Area
This page is still being developed. More content will be added soon!
:::

<!-- TODO: This might be a good place to shamelessly plug the 'Air Gap' course -->
<!-- TODO: The links and information on this page need to be expanded a lot more to really be useful -->

<br />
<br />

## What is Kubernetes?

- [Kubernetes Overview](https://kubernetes.io/docs/concepts/overview/)
  <br />
  <br />

## What is the 'Air Gap'?

<br />
<br />

## What is GitOps?

- [CloudBees GitOps Definition](https://www.cloudbees.com/gitops/what-is-gitops)

## Terms Used

**declarative** &mdash; A user states (via configuration file) which resources are needed and Zarf locates & packages them. A user does not have to know _how_ to download / collect / roll & unroll dependencies for transport, they only have to know _what_ they need.

**package** &mdash; A well-defined (tool-generated / versioned / compressed) collection of software intended for movement (and later use) across a network / adminstrative boundary.

**remote systems** &mdash; Systems organized such that development & maintenance actions occur _primarily_ in locations physically & logically separate from where operations occur.

**constrained systems** &mdash; Systems with explicit resource / adminstrative / capability limitations.

**independent systems** &mdash; Systems organized such that continued operation is possible even when disconnected (temporarily or otherwise) from external systems dependencies.

**air gapped systems** &mdash; Systems designed to operate while _physically disconnected_ from "unsecured" networks like the internet. More on that [here](<https://en.wikipedia.org/wiki/Air_gap_(networking)>).

&nbsp;
