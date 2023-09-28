# Package Sources

Zarf currently supports consuming the following package sources:

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

:::note

There is a special Cluster source available on `inspect` and `remove` that allows for referencing a package via its name:

- `zarf package inspect <package name>`
- `zarf package remove <package name>`

Inspecting a package deployed to a cluster will not be able to show the package's SBOMs, as they are not currently persisted to the cluster.

:::
