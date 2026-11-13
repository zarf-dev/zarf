# stereoscope

<p align="center">
    &nbsp;<a href="https://goreportcard.com/report/github.com/anchore/stereoscope"><img src="https://goreportcard.com/badge/github.com/anchore/stereoscope" alt="Go Report Card"></a>&nbsp;
    &nbsp;<a href="https://github.com/anchore/stereoscope"><img src="https://img.shields.io/github/go-mod/go-version/anchore/stereoscope.svg" alt="GitHub go.mod Go version"></a>&nbsp;
    &nbsp;<a href="https://github.com/anchore/stereoscope/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache-2.0"></a>&nbsp;
    &nbsp;<a href="https://anchore.com/discourse"><img src="https://img.shields.io/badge/Discourse-Join-blue?logo=discourse" alt="Join our Discourse"></a>&nbsp;
</p>

A library for working with container image contents, layer file trees, and squashed file trees.

## Getting Started

See `examples/basic.go`

```bash
docker image save centos:8 -o centos.tar
go run examples/basic.go ./centos.tar
```

Note: To run tests you will need `skopeo` installed.

## Overview

This library provides the means to:
- parse and read images from multiple sources, supporting:
  - docker V2 schema images from the docker daemon, podman, or archive
  - OCI images from disk, directory, or registry
  - images in the local [containers-storage](https://github.com/podman-container-tools/container-libs/tree/main/storage) store (e.g. images built with [buildah](https://buildah.io/) or rootless podman) — see [containers-storage source](#containers-storage-source)
  - singularity formatted image files
- build a file tree representing each layer blob
- create a squashed file tree representation for each layer
- search one or more file trees for selected paths
- catalog file metadata in all layers
- query the underlying image tar for content (file content within a layer)

## containers-storage source

Images that are built with `buildah` (or rootless `podman`) usually live in the local
[containers-storage](https://github.com/podman-container-tools/container-libs/tree/main/storage) store rather than in the docker daemon. The
`containers-storage` source resolves these images directly from the current user's default store, before
falling back to a remote registry pull.

Storage location follows the current process/user via the default containers-storage configuration: rootless
users use their rootless store (typically `~/.local/share/containers/storage`) and root uses the rootful store
(typically `/var/lib/containers/storage`). Stereoscope does not probe both locations; it uses the default store
for the current user.

Usage (assuming the consuming binary, e.g. `syft`, was itself built with the `containers_image_openpgp` build
tag described below — a stock `syft` release build does not include this source):

```bash
# explicit source selection
syft containers-storage:localhost/myimage:latest

# implicit resolution: a plain reference is checked against the local containers-storage
# store before falling back to the OCI registry
syft localhost/myimage:latest
```

> [!NOTE]
> The `containers-storage` source depends on the [image](https://github.com/podman-container-tools/container-libs/tree/main/image) and
> [storage](https://github.com/podman-container-tools/container-libs/tree/main/storage) libraries (the `go.podman.io/image`
> and `go.podman.io/storage` module paths that `github.com/containers/image` and `github.com/containers/storage` have moved
> to) and is only compiled into binaries built with the `containers_image_openpgp` build tag:
>
> ```bash
> go build -tags containers_image_openpgp ./...
> ```
>
> Without that tag, a stub provider keeps the source registered but reports that support was not compiled in, so
> default builds (and downstream consumers) are unaffected. On Linux you may additionally need to exclude the
> cgo graph drivers you do not have headers for, e.g. `-tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper"`.
