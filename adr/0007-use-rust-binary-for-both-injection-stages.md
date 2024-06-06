# 7. Use Rust Binary for Both Injection Stages

Date: 2022-10-29

## Status

Accepted

Amends [3. Image injection into remote clusters without native support](0003-image-injection-into-remote-clusters-without-native-support.md)

## Context

In ADR 3, the decision was made to create a rust binary (`stage1`) that would re-assemble a `registry:2` image and a go registry binary (`stage2`) from a series of configmaps. While this solution works, it is overkill for the operations that `stage2` performs. The `stage2` binary is only responsible for 1. starting a docker registry in `rw` mode, 2. pushing the `registry:2` crane tarball into said registry, 3. starting the docker registry in `r` mode. This `registry:2` image is then immediately consumed by the `zarf-registry` package, creating a true in-cluster docker registry. The injector pod is then destroyed. The overhead this operation creates:

- having to keep track of another binary (making the total number 3 for the zarf ecosystem)
- nearly doubling the amount of configmaps loaded into the cluster (makes init slower)
- having to compile the binary for each platform (adds to the build time + ci)
- using a full-featured docker registry to host a single image (wasteful)

## Decision

The rust binary is already being injected via configmap and unpacking the tarball. There is little need to bring everything but the kitchen sink to just serve a single image. Therefore the decision is to use the rust binary to perform the entire injection process. This required a few changes to the rust binary:

- not only re-assemble + unpack the tarball, but also unpack the `registry:2` image (stored as a [crane tarball format](https://github.com/google/go-containerregistry/tree/main/pkg/v1/tarball))
- transform the `registry:2` crane manifest to a docker v2 manifest
- spin up an HTTP server compliant with the v2 docker registry API to serve the `registry:2` image

## Consequences

The removal of the `stage2` binary makes the build process much simpler, and cuts down on make targets, as well as considerations needed during the build process. Additionally, the init process is faster, as the `stage2` binary is not injected anymore and the `registry:2` image is directly served instead of being pushed to an ephemeral registry, then served. There is a current risk to the new size of the `stage1` binary. Using stable compiler optimizations, the smallest it is able to come to is 868kb. While successfully tested on `k3s`, `kind`, and `k3d`, it is possible that the binary is too large for some Kubernetes distros (due to the potential interpretation of the configmap size limit). Additionally, the docker API implementation only serves a v2 manifest (most likely OCI in a future iteration) / image pull flow. This also greatly increases the lines of rust code and logic used within this repo, and future changes to this code will require more rust knowledge and experience, especially understanding the docker registry API and docker manifest formats.
