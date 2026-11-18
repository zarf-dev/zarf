# Copilot Instructions

## Project Overview

ORAS Go is a Go SDK for OCI artifact management and serves as an OCI registry client. Provides unified APIs for push/pull operations across OCI registries, file systems, and memory stores. Compliant with [OCI Image Format Specification](https://github.com/opencontainers/image-spec) (defines the schema for container images: manifests, image indexes, filesystem layers, and configuration) and [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec) (defines an API protocol to facilitate and standardize the distribution of content). ORAS is expected to support the latest versions of both specs (currently v1.1.1).

**Stack:** Go, OCI specifications

**Critical Design Principles:**
- **Content-Addressable Storage (CAS):** All content is addressed by cryptographic digests (descriptors), enabling reliable deduplication and verification.
- **Graph-based model:** Unlike other OCI clients, ORAS models every element of an artifact as nodes in a Directed Acyclic Graph (DAG). See [ORAS Graphs](https://oras.land/docs/client_libraries/overview#graphs).
- **Copy-based operations:** ORAS models data movement as copy operations rather than separate push/pull. See [Unified Experience](https://oras.land/docs/client_libraries/overview#unified-experience).

## Key Guidelines

1. **Public APIs:** Document all exported functions/types. **Think carefully before exporting** - exported symbols become public API and cannot be removed until a major version bump. Keep exports minimal.
2. **Testing:** Include unit tests for new functionality (`*_test.go`)
3. **Descriptors:** Use OCI descriptors for content identification
4. **Error handling:** Return descriptive errors with context
5. **Concurrency:** Use `internal/syncutil` patterns
6. **Go versions:** Support 2 latest releases (see `go.mod`)

## Code Standards

- Follow Go idioms and best practices from [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `make test` before commits (includes race detection + coverage)
- Maintain 80%+ test coverage for changes (required for CI to pass)
- Vendor dependencies via `make vendor`
- No CRLF line endings (`make check-encoding`)
- Apache 2.0 license header required on all source files (checked via `.github/workflows/license-checker.yml`)

## Patterns

- **Targets:** Abstract storage backends (registry/file/memory)
- **Copy operations:** Primary API for artifact transfer
- **Content stores:** Content-addressable storage pattern
- **Graph operations:** Handle artifact dependencies

## Configuration

Existing APIs mix several configuration styles. New code follows these rules:

- **Library functions** take an options struct (`XOptions`) whose zero value means "use the defaults".
- **Client construction** goes through `remote.ClientBuilder`.
- **New APIs** don't add functional options. Existing ones, such as `policy.NewEvaluator`, stay as they are.

## Repository Structure

**Key directories** (not exhaustive):

```
oras-go/
├── content/                    # Content management & storage
│   ├── file/                   # File-based storage
│   ├── memory/                 # In-memory storage
│   └── oci/                    # OCI layout storage
├── registry/                   # Registry interfaces & operations
│   └── remote/                 # Remote registry client
│       ├── auth/               # Authentication (OAuth2, basic)
│       ├── credentials/        # Credential stores (Docker config)
│       ├── errcode/            # OCI error codes
│       └── retry/              # Retry policies
├── internal/                   # Non-public utilities
│   ├── cas/                    # Content-addressable storage
│   ├── copyutil/               # Copy stack utilities
│   ├── graph/                  # Dependency graph
│   └── syncutil/               # Synchronization primitives
├── errdef/                     # Error definitions
├── docs/                       # Documentation
└── scripts/                    # Build & test scripts
```

**Core files:** `content.go`, `copy.go`, `pack.go`, `registry/registry.go`

## Common Tasks

```bash
make test      # Test with coverage
make covhtml   # View coverage report
go test ./...  # Run all tests
```
