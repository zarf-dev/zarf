# 20. Package Sources

Date: 2023-09-28

## Status

Accepted

## Context

Zarf natively supports creating the following package sources:

- Local Tarball (`.tar` and `.tar.zst`)
  - Via `zarf package create <dir> -o <dir>`, with compression determined by `metadata.uncompressed` in `zarf.yaml`
- Split Tarball (`.part...`)
  - Via `zarf package create <dir> --max-package-size <size> -o <dir>`
- OCI package (`oci://`)
  - Via `zarf package publish <source> oci://` or `zarf package create <dir> -o oci://...`
- In-cluster (Deployed) package
  - Post `zarf package deploy <source>` the package is show in `zarf package list`

However, the current loading abilities of Zarf have been inconsistent depending upon the action specified. For example:

- Split tarball packages could be created, deployed, but not inspected, or removed
- In-cluster packages could be removed (by name), but not inspected
- HTTPs URLs could be deployed, but not inspected, or removed
- etc...

## Decision

Zarf must support the `deploy`, `inspect`, `remove`, `publish`, `pull`, and `mirror-resources` commands across package sources.

For common behavior to be exhibited by all sources, the `PackageSource` interface has been introduced along with the `layout` library.

```go
// src/pkg/packager/sources/new.go

// PackageSource is an interface for package sources.
//
// While this interface defines three functions, LoadPackage, LoadPackageMetadata, and Collect; only one of them should be used within a packager function.
//
// These functions currently do not promise repeatability due to the side effect nature of loading a package.
type PackageSource interface {
    // LoadPackage loads a package from a source.
    //
    // For the default sources included in Zarf, package integrity (checksums, signatures, etc.) is validated during this function
    // and expects the package structure to follow the default Zarf package structure.
    //
    // If your package does not follow the default Zarf package structure, you will need to implement your own source.
    LoadPackage(*layout.PackagePaths) error
    // LoadPackageMetadata loads a package's metadata from a source.
    //
    // This function follows the same principles as LoadPackage, with a few exceptions:
    //
    // - Package integrity validation will display a warning instead of returning an error if
    //   the package is signed but no public key is provided. This is to allow for the inspection and removal of packages
    //   that are signed but the user does not have the public key for.
    LoadPackageMetadata(dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) error

    // Collect relocates a package from its source to a tarball in a given destination directory.
    Collect(destinationDirectory string) (tarball string, err error)
}
```

The following sources have been implemented:

- Local Tarball (`.tar` and `.tar.zst`)
- Split Tarball (`.part...`)
- HTTP(S) URL
- Published OCI package (`oci://`)
- In-cluster (Deployed) package (`inspect` and `remove` only)

The `layout` library contains the `PackagePaths` struct which supersedes the prior `TempPaths` struct. This new struct contains access methods to different aspects of Zarf's internal package layout. This struct is passed to the `PackageSource` functions to allow for the loading of packages into the correct layout. In order for a package to be loaded into the correct layout, the package must follow the default Zarf package structure, or be converted to the expected structure during loading operations.

## Consequences

The `PackageSource` interface and `layout` library are now part of the public API of Zarf. This means that any package source can be implemented by a third party and used with Zarf as a first class citizen.

By moving towards a behavioral driven design, Zarf is now more consistent in its behavior across all package sources. If it walks like a source, and it quacks like a source, it's a source.
