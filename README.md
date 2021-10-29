# Zarf - Kubernetes Airgap Buddy

<img align="right" alt="zarf logo" src=".images/zarf-logo.png"  height="256" />

Zarf massively simplifies the setup & administration of kubernetes clusters "across the [air gap](https://en.wikipedia.org/wiki/Air_gap_(networking))".

It provides a static go binary CLI that can pull, package, and install all the things your clusters need to run.  It caches downloads (for speed), hashes packages (for security), and can even _install the kubernetes cluster itself_ if you want it to.

Zarf runs on [a bunch of operating systems](./docs/supported-oses.md) and aims to support configurations ranging from "I want to run one, simple app" to "I need to support & dependency control a _bunch_ of internet-disconnected clusters".

&nbsp;

> _This repo is in transition from [Repo1](https://repo1.dso.mil/) by [DoD Platform One](http://p1.dso.mil/) to [Github](https://github.com/defenseunicorns/zarf).  See [the announcments post](https://github.com/defenseunicorns/zarf/discussions/1#discussion-3560306) for the latest URLs for this project during this transition._

&nbsp;

&nbsp;

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

  Experience just how easy it is to go from _**zero** to **chainsaw wielding hero** (of the Kubernetes cluster)_ using Zarf!
  
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

  Sometimes running your app in-cluster is enough&mdash;usually, it's not. Find out how to inject commonly-required, uber-useful functionality through the use of Zarf components.

  </td>
  <td>

You will also need to configure the .env file, use the command below to generate a template.  _Note that you don't need to set RHEL creds if you aren't using RHEL_

`earthly +envfile`

## Building
---
To build the packages needed for RHEL-based distros, you will need a Red Hat account (developer accounts are free) to pull the required RPMs for SELINUX-enforcing within the environment.  You must specify the credentials along with the RHEL version flag (7 or 8) in the .env file.  To build the package:

### Step 1b - Configure the `.env` file

Some secrets also have to be passed to Earthly for your build, these are stored in the `.env` file.  YOu can generate a template to complete with the command below. 

`earthly +envfile`

_To build the packages needed for RHEL-based distros, you will need to use your RedHat Developer account to pull the required RPMs for SELINUX-enforcing within the environment.  You must specify your credentials along with a RHEL version flag (7 or 8) in the `.env` file_

  Coming Soon!

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>
  
  **Disconnected GitOps**

  _The Utility Cluster_
  
  </td>
  <td>

  Zarf overcomes the "the Air Gap problem" using a Kubernetes cluster (and k8s-native tooling) for the care & feeding of _other k8s clusters_.
  
  Here's how it works and what ops/support looks like.

  </td>
  <td>

  Coming Soon!

  </td>
  </tr>
<!-- row end -->

</tbody>
</table>

&nbsp;


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
  
  Thinking about hacking on the Zarf binary itself? Or, perhaps you want to run the examples in an isolated environment (the same way we do)? Get your machine setup _just right_ using these instructions!

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
  
  You've got a development workstation setup, so... now what?  Why not _build your own Zarf_? Step-by-step instructions, here.

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
  
  As with most collaborative efforts there are guidelines for contributing to the project. Find out what they are & how to make them work, here.

  </td>
  <td>

  [Read](./CONTRIBUTING.md)

  </td>
  </tr>
<!-- row end -->

</tbody>
</table>

&nbsp;


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
  
  Zarf is intended to run on a variety of Operating Systems&mdash;you can find out which _and_ discover how to take Zarf for a test-drive (in a VM of your favorite flavor) by clicking the link!
  
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
  
  Need to understand what's happening in your cluster? Zarf can give you visibility by injecting a _Logging_ component.  Looking for some additional CLI tooling? Install the _Management_ component.
  
  Find out all the other stuff Zarf offers, here.
  
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

  Zarf can build deployment packages using images pulled from pretty much anywhere... but for gov't-approved & "[hardened](https://en.wikipedia.org/wiki/Hardening_(computing))" container images check out Platform One's [Iron Bank](https://p1.dso.mil/#/products/iron-bank/).
  
  </td>
  <td>

  [Read](./docs/ironbank.md)

  </td>
  </tr>
<!-- row end -->

</tbody>
</table>


&nbsp;
