# 2. Image injection into remote clusters without native support

Date: 2022-03-04

## Status

Accepted

## Context

In order to create any workloads in K8s, an image has to exist on the node or be pulled in from an OCI Distribution server (docker registry). Local K8s distros such as KIND, K3D, K3S, Microk8s have CLI support for injecting or pushing images into the CRIs of cluster nodes.  No standard or tool exists to do this generically in K8s as the CRI sits outside the problem of K8s beyond just communicating with it.  In order for Zarf to push images into a K8s cluster that does not have a CLI option to inject the image, we have to establish some mechanism to create a temporary registry, use an existing one or some other way inject the image in the cluster.  Zarf has to support unknown environments with no other dependencies, we cannot assume a registry exists.  Normally when K8s pulls images from a remote registry, it sends the request to the CRI that then does the pull outside of the K8s context.  This runs at the host-level on a per-node basis so a private registry is created the TLS trust chain needs to be then modified on any host/node that would attempt to pull the image.  The two primary ways to do this are modifying the node's root certificate authorities or the CRIs configuration if it has an option for TLS root CA.  Lastly, as this is per-node, all nodes in the cluster would need to be modified or some type of affinity/taint to force the pod to use a single node during bootstrapping.

## Decision

Because we cannot require third-party dependencies, the requirement from some cloud-managed or distro-managed registry is not an option as not every K8s cluster will have that available.  Running an in-memory registry while zarf is performing `zarf init` was initially explored as it would resolve the external dependency issue.  However, this solution proved to be too complex when dealing with network traversal (especially behind NATs), firewall and NACL rulesets.  Additionally, because of the complexities around TLS trust, the in-memory registry was not a viable solution.

@todo: explain decision around the injector work


## Consequences

@todo: replace me
What becomes easier or more difficult to do and any risks introduced by the change that will need to be mitigated.
