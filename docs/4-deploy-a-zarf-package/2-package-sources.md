# Package Sources

Zarf supports interacting with Zarf packages from a variety of sources. For library users of Zarf looking to implement their own, please refer to the `PackageSource` interface in `src/pkg/packager/sources/new.go`.

Zarf currently supports the following package sources:

- Local Tarball (`.tar` and `.tar.zst`)
- Split Tarball (`.part...`)
- HTTP(S) URL
- Published OCI package (`oci://`)

The following commands accept a source as their first argument:

- `zarf package deploy <source>`
- `zarf package inspect <source>`
- `zarf package remove <source>`
- `zarf package publish <source>`
- `zarf package pull <source>`
- `zarf package mirror-resources <source>`

There is a special Cluster source available on `inspect` and `remove` that allows for referencing a package via its name:

- `zarf package inspect <package name>`
- `zarf package remove <package name>`
