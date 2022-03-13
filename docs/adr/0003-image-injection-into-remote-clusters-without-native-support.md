# 3. Image injection into remote clusters without native support

Date: 2022-03-04

## Status

Accepted

## Context

In order to create any workloads in K8s, an image has to exist on the node or be pulled in from an OCI Distribution server (docker registry). Local K8s distros such as KIND, K3D, K3S, Microk8s have CLI support for injecting or pushing images into the CRIs of cluster nodes. No standard or tool exists to do this generically in K8s as the CRI sits outside the problem of K8s beyond just communicating with it. For Zarf to push images into a K8s cluster that does not have a CLI option to inject the image, we have to establish some mechanism to create a temporary registry, use an existing one, or inject the image in the cluster. Zarf must also support unknown environments with no other dependencies; we cannot assume a registry exists. Typically when K8s pulls images from a remote registry, it sends the request to the CRI that then does the pull outside of the K8s context. The CRI runs at the host level on a per-node basis, so to access a private registry, the TLS trust needs to be modified on any host/node that would attempt to pull the image. The two primary ways to do this are modifying the node's root certificate authorities or the CRIs configuration if it has an option for TLS root CA. Lastly, as this is per node, all nodes in the cluster would need to be modified or some affinity/taint to force the pod to use a single node during bootstrapping.

## Decision

Because we cannot require third-party dependencies, the requirement from some cloud-managed or distro-managed registry is not an option as not every K8s cluster will have that available.  Running an in-memory registry while zarf is performing `zarf init` was initially explored to resolve the external dependency issue.  However, this solution proved too complex when dealing with network traversal (especially behind NATs), firewall, and NACL rulesets.  Additionally, because of the complexities around TLS trust, the in-memory registry was not a viable solution.

After iterating on several solutions, including `kubectl cp`, `kubectl exec`, a busybox configmap injection that brought in netcat to stream files, and a few variations on those ideas, we found using only configmaps to be the most predictable method for injection. The challenge with this mode is that the ETCD limit is 1 MB total, with binary data base64-encoded leaving roughly 670 KBs of data per configmap, far too small to bring the registry tarball in a single configmap. So instead, we used a combination of a custom Rust binary compiled with MUSL to perform file concatenation, untarring, and verification of a set of split tarball chunks across a variable number of configmaps. We found that chunking by less than the near-maximum configmap size was more performant on some resource-constrained control planes. The details of this solution and a drawing are located in the main README of this repo.

## Consequences

Because this mechanism uses only a standard Podspec and Configmap definition, there should be no problem running this on any generic K8s cluster.  However, the current implementation requires manually compiling the Rust binary, and mini go binary for injection; these binaries are also committed to the git repo.  Future updates should move these two compile actions to pipeline stages to eliminate stale binaries.  The other consideration is that this brings in a new language; even if it is only for a few dozen lines of code, it is a context switch for the engineering team should an update need to be made.
