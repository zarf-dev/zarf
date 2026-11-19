# ORAS Go library

<p align="left">
<a href="https://oras.land/"><img src="https://oras.land/img/oras.svg" alt="ORAS logo" width="100px"></a>
</p>

`oras-go` is a Go library for managing OCI artifacts, compliant with the [OCI Image Format Specification](https://github.com/opencontainers/image-spec) and the [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec). It provides unified APIs for pushing, pulling, and managing artifacts across OCI-compliant registries, local file systems, and in-memory stores.

> [!Note]
> The `main` and `v2` branches follow [Go's Security Policy](https://github.com/golang/go/security/policy) and support the two latest versions of Go (currently `1.26` and `1.27`).
 
## Getting Started

### Concepts

Gain insights into the fundamental concepts:

- [Modeling Artifacts](docs/Modeling-Artifacts.md)
- [Targets and Content Stores](docs/Targets.md)

### Quickstart

Follow the step-by-step tutorial to use `oras-go` v3:

- [Quickstart: Managing OCI Artifacts with `oras-go` v3](docs/tutorial/quickstart.md)

### Examples

Check out sample code for common use cases of `oras-go` v3:

- [Artifact copying](https://pkg.go.dev/github.com/oras-project/oras-go/v3#pkg-examples)
- [Registry operations](https://pkg.go.dev/github.com/oras-project/oras-go/v3/registry#pkg-examples)
- [Repository operations](https://pkg.go.dev/github.com/oras-project/oras-go/v3/registry/remote#pkg-examples)
- [Authentication](https://pkg.go.dev/github.com/oras-project/oras-go/v3/registry/remote/auth#pkg-examples)
- [Credentials management](https://pkg.go.dev/github.com/oras-project/oras-go/v3/registry/remote/credentials#pkg-examples)

Find more API examples at [pkg.go.dev](https://pkg.go.dev/github.com/oras-project/oras-go/v3).

## Versioning

This project follows [Semantic Versioning](https://semver.org/) (`MAJOR`.`MINOR`.`PATCH`), with `MAJOR` for breaking changes, `MINOR` for backward-compatible features, and `PATCH` for backward-compatible fixes.

## Branches

- To migrate from `v2` to `v3`, see [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md).
- To migrate from `v1` to `v2`, see the [`v2` branch migration guide](https://github.com/oras-project/oras-go/blob/v2/MIGRATION_GUIDE.md).

### main (v3 development)

[![Build Status](https://github.com/oras-project/oras-go/actions/workflows/build.yml/badge.svg?event=push&branch=main)](https://github.com/oras-project/oras-go/actions/workflows/build.yml?query=workflow%3Abuild+event%3Apush+branch%3Amain)
[![codecov](https://codecov.io/gh/oras-project/oras-go/branch/main/graph/badge.svg)](https://codecov.io/gh/oras-project/oras-go)

The [`main`](https://github.com/oras-project/oras-go/tree/main) branch is under active development for `v3` and may contain breaking changes. **Not recommended for production use.**

### v2 (stable)

[![Build Status](https://github.com/oras-project/oras-go/actions/workflows/build.yml/badge.svg?event=push&branch=v2)](https://github.com/oras-project/oras-go/actions/workflows/build.yml?query=workflow%3Abuild+event%3Apush+branch%3Av2)
[![codecov](https://codecov.io/gh/oras-project/oras-go/branch/v2/graph/badge.svg)](https://codecov.io/gh/oras-project/oras-go)
[![Go Report Card](https://goreportcard.com/badge/oras.land/oras-go/v2)](https://goreportcard.com/report/oras.land/oras-go/v2)
[![Go Reference](https://pkg.go.dev/badge/oras.land/oras-go/v2.svg)](https://pkg.go.dev/oras.land/oras-go/v2)

The [`v2`](https://github.com/oras-project/oras-go/tree/v2) branch contains the latest stable release and is **recommended for production use**.

New features and bug fixes from `main` will be backported to `v2` if applicable.

### v1 (maintenance)

[![Build Status](https://github.com/oras-project/oras-go/actions/workflows/build.yml/badge.svg?event=push&branch=v1)](https://github.com/oras-project/oras-go/actions/workflows/build.yml?query=workflow%3Abuild+event%3Apush+branch%3Av1)
[![codecov](https://codecov.io/gh/oras-project/oras-go/branch/v1/graph/badge.svg)](https://codecov.io/gh/oras-project/oras-go)
[![Go Report Card](https://goreportcard.com/badge/oras.land/oras-go)](https://goreportcard.com/report/oras.land/oras-go)
[![Go Reference](https://pkg.go.dev/badge/oras.land/oras-go.svg)](https://pkg.go.dev/oras.land/oras-go)

The [`v1`](https://github.com/oras-project/oras-go/tree/v1) branch is in maintenance mode and receives only dependency updates and security fixes. No new features are planned.

## Community

- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Security Policy: [SECURITY.md](SECURITY.md)
- Reviewing Guide: [Reviewing Guide](https://github.com/oras-project/community/blob/main/REVIEWING.md)
- Slack: [`#oras`](https://cloud-native.slack.com/archives/CJ1KHJM5Z) channel on CNCF Slack
