# Zarf - Kubernetes Air Gap Buddy 

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

  [Read](./examples/game/add-logging.md)

  </td>
  </tr>
<!-- row end -->

<!-- row start -->
  <tr valign="top">
  <td>

  **Roll Your Own**

  _Custom packages_

  </td>
  <td>

  Once you're comfortable with the basic workflow & able to deploy _someone else's_ Zarf deployment packages, it's time to roll your own.  Here's how.

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
# This block is about expected USECASES & ADMIN of Zarf (in production)
##########
-->
## To understand *the different modes of use*, have a look at...

<table>
<tbody>

<!-- row start: cuz markdown hates html indention -->
  <tr valign="top">
  <td width="150">

  **Simple Applications**

  _Appliance Mode_

  </td>
  <td>

  If want to "run a Kubernetes app" but aren't into hand-rolling a cluster just for it, Zarf can help. Here's how, and _why_ you might want to.

  </td>
  <td>

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
