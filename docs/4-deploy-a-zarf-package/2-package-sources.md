# Package Sources

Zarf currently supports consuming the following package sources:

- Local Tarball Path (`.tar` and `.tar.zst`) - The default output on `zarf package create`
- Split Tarball Path (`.part...`) - Optionally created by specifying `--max-package-size`
- Remote Tarball URL (`http://` and `https://` ) - A package tarball hosted on a web server
- Remote OCI Reference (`oci://`) - A package published to an OCI compatible registry

A source can be used with the following commands as their first argument:

- `zarf package deploy <source>`
- `zarf package inspect <source>`
- `zarf package remove <source>`
- `zarf package publish <source>`
- `zarf package pull <source>`
- `zarf package mirror-resources <source>`

:::note

In addition to the traditional sources, there is also a special Cluster source available on `inspect` and `remove` that allows for referencing a package via its name:

- `zarf package inspect <package name>`
- `zarf package remove <package name>`

Inspecting a package deployed to a cluster will not be able to show the package's SBOMs, as they are not currently persisted to the cluster.

:::
