# cyclonedx-go

[![Build Status](https://github.com/CycloneDX/cyclonedx-go/actions/workflows/ci.yml/badge.svg)](https://github.com/CycloneDX/cyclonedx-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/CycloneDX/cyclonedx-go)](https://goreportcard.com/report/github.com/CycloneDX/cyclonedx-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/nscuro/cyclonedx-go.svg)](https://pkg.go.dev/github.com/CycloneDX/cyclonedx-go)
[![License](https://img.shields.io/badge/license-Apache%202.0-brightgreen.svg)](LICENSE)  
[![Website](https://img.shields.io/badge/https://-cyclonedx.org-blue.svg)](https://cyclonedx.org/)
[![Slack Invite](https://img.shields.io/badge/Slack-Join-blue?logo=slack&labelColor=393939)](https://cyclonedx.org/slack/invite)
[![Group Discussion](https://img.shields.io/badge/discussion-groups.io-blue.svg)](https://groups.io/g/CycloneDX)
[![Twitter](https://img.shields.io/twitter/url/http/shields.io.svg?style=social&label=Follow)](https://twitter.com/CycloneDX_Spec)

*cyclonedx-go is a Go library to consume and produce CycloneDX Software Bill of Materials (SBOM)*

> If you just want to create BOMs for your Go projects, see [*cyclonedx-gomod*](https://github.com/CycloneDX/cyclonedx-gomod)

## Installation

```
go get github.com/CycloneDX/cyclonedx-go
```

## Usage

Please refer to the module's [documentation](https://pkg.go.dev/github.com/CycloneDX/cyclonedx-go#section-documentation).  
Also, checkout the [`examples`](./example_test.go) to get an idea of how this library may be used.

## Compatibility

| cyclonedx-go versions | Supported Go versions | Supported CycloneDX spec |
|:---------------------:|:---------------------:|:------------------------:|
|       < v0.4.0        |         1.14+         |           1.2            |
|       == v0.4.0       |         1.14+         |           1.3            |
|  >= v0.5.0, < v0.7.0  |         1.15+         |           1.4            |
|  >= v0.7.0, < v0.8.0  |         1.17+         |         1.0-1.4          |
|       == v0.8.0       |         1.18+         |         1.0-1.5          |
|       >= v0.9.0       |         1.20+         |         1.0-1.6          |
|       >= 0.10.0       |         1.23+         |         1.0-1.6          |

We're aiming to support all [officially supported](https://golang.org/doc/devel/release.html#policy) Go versions, plus
an additional older version.

Prior to v0.7.0, this library only supported the latest version of the CycloneDX specification. While it is generally 
possible to *read* BOMs of an older spec, *writing* would exclusively produce BOMs conforming to the latest supported spec.

Starting with v0.7.0, writing BOMs conforming to all previous version of the spec is also possible.

## Copyright & License

CycloneDX Go is Copyright (c) OWASP Foundation. All Rights Reserved.

Permission to modify and redistribute is granted under the terms of the Apache 2.0 license.  
See the [LICENSE](./LICENSE) file for the full license.

## Contributing

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/CycloneDX/cyclonedx-go)

Pull requests are welcome. But please read the
[CycloneDX contributing guidelines](https://github.com/CycloneDX/.github/blob/master/CONTRIBUTING.md) first.

It is generally expected that pull requests will include relevant tests. Tests are automatically run against all
supported Go versions (see [Compatibility](#compatibility)) for every pull request.
