# Zarf - DevSecOps for Air Gap Systems

<img align="right" alt="zarf logo" src=".images/zarf-logo.png"  height="256" />

Zarf simplifies the setup & administration of Kubernetes clusters, cyber systems & workloads that support DevSecOps "across the [air gap](https://en.wikipedia.org/wiki/Air_gap_(networking))."

It provides a static go binary (can run anywhere) CLI that can pull, package, and install all the things your clusters need to run and any necessary resources to standup infrastructure such as Terraform. Zarf also caches downloads (for speed), hashes packages (for security), and can _even install the Kubernetes cluster itself_ if you needed.

Zarf runs on [a bunch of operating systems](./docs/supported-oses.md). It aims to support configurations ranging from "I want to run one, simple app" to "I need to support & dependency control a _bunch_ of internet-disconnected clusters."

Zarf was theorized and initially demonstrated in Naval Postgraduate School research:

[A DEVSECOPS APPROACH FOR DEVELOPING AND DEPLOYING CONTAINERIZED CLOUD-BASED SOFTWARE ON SUBMARINES](https://calhoun.nps.edu/handle/10945/68688).


[![asciicast](https://asciinema.org/a/475530.svg)](https://asciinema.org/a/475530)

## Why is Zarf Needed?
Most of the software ecosystem assumes your systems have access to the internet. The world (for good reasons) has become more and more dependent upon Software as a Service (SaaS), which assumes a robust connection to the internet and a willingness to inherently trust 3rd party providers. Although this makes sense for most of the world, certain secure systems must operate either fully disconnected, semi-disconnected, or might need the ability to disconnect in case of emergencies (like while under an active cyber attack). Although only a small percentage of systems, these secure systems make up some of the most vital systems globally, such as Aerospace and Defense, Finance, Healthcare, Energy, Water, Sewage, and many Federal, Local, and State Government systems.  

## Explain Zarf Like I'm Ten(ish)

Zarf allows you to bundle portions of "the internet" into a single package to be installed later following specific instructions. A Zarf package is just a single file that includes everything you need to manage a system or capability while fully disconnected. Think of a disconnected system as a system that always is or sometimes is on airplane mode.

You bring this single file (we call it a package) to the system you want to install or update new software on. The package includes instructions on how to assemble all the pieces of software (components) once on the other side. These instructions are fully "declarative," which means that everything is represented by code and automated instead of manual. The hardest part is assembling the declarative package on the connected side. But once everything is packaged, Zarf makes even very complex systems easy to install, update, and maintain within disconnected systems.

Such packages also become highly distributable, as they can now run on edge, embedded systems, secure cloud, data centers, or even in a local environment. This is incredibly helpful for organizations that need to integrate and deploy software from multiple, secure development environments from disparate development teams into disconnected IT operational environments. Zarf helps ensure that development teams can integrate with the production environment they are deploying to, even if they will never actually touch that environment.

Zarf makes DevSecOps for air gap possible.

<!--
##########
# This block is about LEARNING TO USE Zarf
##########
-->
## If you're *just getting into Zarf*, you should...

<table>
<tbody>

<!-- row start: cuz markdown hates html indention -->
  <tr valign="top">
  <td width="150">

  **Get Started**

  _Using Zarf_

  </td>
  <td>

  Experience just how easy it is to go from _**zero** to **chainsaw-wielding hero** (of the Kubernetes cluster)_ using Zarf!

  </td>
  <td>

  [Read](./examples/game/)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Add Logging**

  _Zarf components_

  </td>
  <td>

  Sometimes running your app in-cluster is enough&mdash; usually, it's not. Find out how to inject commonly-required, uber-useful functionality through Zarf components.

  </td>
  <td>

  [Read](./examples/game/add-logging.md)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Join Slack Channels**

  </td>
  <td>

  Join our project channels on K8s slack. <a href="https://kubernetes.slack.com/archives/C03B6BJAUJ3">#zarf</a> <a href="https://kubernetes.slack.com/archives/C03BP9Z3CMA">#zarf-dev</a>


  </td>
  <td>

 https://slack.k8s.io/

  </td>
  </tr>
<!-- row end -->
</tbody>
</table>

<!--
##########
# This block is about DEVELOPING Zarf
##########
-->
## If you'd rather *help develop Zarf*, you should read...

<table>
<tbody>

<!-- row start: cuz markdown hates html indention -->
  <tr valign="top">
  <td width="150">

  **Workstation Setup**

  </td>
  <td>

  Are you thinking about hacking on the Zarf binary itself? Or, perhaps you want to run the examples in an isolated environment (the same way we do)? Then, get your machine setup _just right_ using these instructions!

  </td>
  <td>

  [Read](./docs/workstation.md)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Build Your First Zarf**

  </td>
  <td>

  You've got a development workstation setup, so... now what? Why not _build your own Zarf_? Step-by-step instructions here.

  </td>
  <td>

  [Read](./docs/first-time-build.md)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Contribution Guide**

  </td>
  <td>

  As with most collaborative efforts, there are guidelines for contributing to the project. Find out what they are & how to make them work here.

  </td>
  <td>

  [Read](./CONTRIBUTING.md)

  </td>
  </tr>
<!-- row end -->

</tbody>
</table>

<!--
##########
# This block is about the MINUTIA & UNDERSTANDING WHY Zarf is the way it is
##########
-->
## Or, for *details & design decisions*, check out...

<table>
<tbody>

<!-- row start: cuz markdown hates html indention -->
  <tr valign="top">
  <td width="150">

  **Supported OSes**

  </td>
  <td>

  Zarf is intended to run on a variety of Operating Systems&mdash; you can find out which _and_ discover how to take Zarf for a test drive (in a VM of your favorite flavor) by clicking the link!

  </td>
  <td>

  [Read](./docs/supported-oses.md)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Zarf Components**

  </td>
  <td>

  Need to understand what's happening in your cluster? Zarf can give you visibility by injecting a _Logging_ component.

  Find out all the other stuff Zarf offers here.

  </td>
  <td>

  [Read](./docs/components.md)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Usage Examples**

  </td>
  <td>

  There are a bunch of interesting ways to use Zarf beyond "space-marine-ing" your way through _the_ pixel demon invasion. Browse our examples directory to find out what other neat things are available.

  </td>
  <td>

  [Read](./examples)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Iron Bank** <br/>

  _Hardened image registry_

  </td>
  <td>

  Zarf can build deployment packages using images pulled from pretty much anywhere...but for gov't-approved & "[hardened](https://en.wikipedia.org/wiki/Hardening_(computing))" container images, check out Platform One's [Iron Bank](https://p1.dso.mil/#/products/iron-bank/).

  </td>
  <td>

  [Read](./docs/ironbank.md)

  </td>
  </tr>
<!-- row end -->

</tbody>
</table>


&nbsp;


## Zarf Nerd Notes

Zarf is written entirely in [go](https://go.dev/), except for a single 400Kb binary for the injector system written in [rust](https://www.rust-lang.org/), so we can fit it in a [configmap](https://kubernetes.io/docs/concepts/configuration/configmap/). All assets are bundled together into a single [zstd](https://facebook.github.io/zstd/) tarball on each `zarf package create` operation. On the air gap / offline side, `zarf package deploy` extracts the various assets and places them on the filesystem or installs them in the cluster, depending on what the [zarf package](Zarf.yaml) says to do. Some important ideas behind Zarf:

- All workloads are installed in the cluster via the [Helm SDK](https://helm.sh/docs/topics/advanced/#go-sdk)
- The OCI Registries used are both from [Docker](https://github.com/distribution/distribution)
- Currently, the Registry and Git servers _are not HA_, see [#375](https://github.com/defenseunicorns/zarf/issues/376) and [#376](https://github.com/defenseunicorns/zarf/issues/376) for discussion on this
- To avoid TLS issues, Zarf binds to `127.0.0.1:31999` on each node as a [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport) to allow all nodes to access the pod(s) in the cluster
- Until [#306](https://github.com/defenseunicorns/zarf/pull/306) is merged, during helm install/upgrade a [Helm PostRender](https://helm.sh/docs/topics/advanced/#post-rendering) function is called to mutate images and [ImagePullSecrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod) so the deployed resources use the NodePort binding
- Zarf uses a custom injector system to bootstrap a new cluster. See the PR [#329](https://github.com/defenseunicorns/zarf/pull/329) and [ADR](docs/adr/0003-image-injection-into-remote-clusters-without-native-support.md) for more details on how we came to this solution.  The general steps are listed below:
  - Get a list of images in the cluster
  - Attempt to create an ephemeral pod using an image from the list
  - A small rust binary that is compiled using [musl](https://www.musl-libc.org/) to keep the size the max binary size of ~ 672 KBs is injected into the pod
  - The mini zarf registry binary and `docker:2` images are put in a tar archive and split into 512 KB chunks; larger sizes tended to cause latency issues on low-resource control planes
  - An init container runs the rust binary to re-assemble and extract the zarf binary and registry image
  - The container then starts and runs the zarf binary to host the registry image in an embedded docker registry
  - After this, the main docker registry chart is deployed, pulls the image from the ephemeral pod, and finally destroys the created configmaps, pod, and service

&nbsp;
### Zarf Architecture
![Architecture Diagram](./docs/architecture.drawio.svg)


[Source DrawIO](docs/architecture.drawio.svg)
