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
  - singularity formatted image files
- build a file tree representing each layer blob
- create a squashed file tree representation for each layer
- search one or more file trees for selected paths
- catalog file metadata in all layers
- query the underlying image tar for content (file content within a layer)
