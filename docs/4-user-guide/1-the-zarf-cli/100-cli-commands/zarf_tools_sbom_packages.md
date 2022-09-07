## zarf tools sbom packages

Generate a package SBOM

### Synopsis

Generate a packaged-based Software Bill Of Materials (SBOM) from container images and filesystems

```
zarf tools sbom packages [SOURCE] [flags]
```

### Examples

```
  syft packages alpine:latest                                a summary of discovered packages
  syft packages alpine:latest -o json                        show all possible cataloging details
  syft packages alpine:latest -o cyclonedx                   show a CycloneDX formatted SBOM
  syft packages alpine:latest -o cyclonedx-json              show a CycloneDX JSON formatted SBOM
  syft packages alpine:latest -o spdx                        show a SPDX 2.2 Tag-Value formatted SBOM
  syft packages alpine:latest -o spdx-json                   show a SPDX 2.2 JSON formatted SBOM
  syft packages alpine:latest -vv                            show verbose debug information
  syft packages alpine:latest -o template -t my_format.tmpl  show a SBOM formatted according to given template file

  Supports the following image sources:
    syft packages yourrepo/yourimage:tag     defaults to using images from a Docker daemon. If Docker is not present, the image is pulled directly from the registry.
    syft packages path/to/a/file/or/dir      a Docker tar, OCI tar, OCI directory, or generic filesystem directory

  You can also explicitly specify the scheme to use:
    syft packages docker:yourrepo/yourimage:tag          explicitly use the Docker daemon
    syft packages podman:yourrepo/yourimage:tag        	 explicitly use the Podman daemon
    syft packages registry:yourrepo/yourimage:tag        pull image directly from a registry (no container runtime required)
    syft packages docker-archive:path/to/yourimage.tar   use a tarball from disk for archives created from "docker save"
    syft packages oci-archive:path/to/yourimage.tar      use a tarball from disk for OCI archives (from Skopeo or otherwise)
    syft packages oci-dir:path/to/yourimage              read directly from a path on disk for OCI layout directories (from Skopeo or otherwise)
    syft packages dir:path/to/yourproject                read directly from a path on disk (any directory)
    syft packages file:path/to/yourproject/file          read directly from a path on disk (any single file)

```

### Options

```
      --catalogers stringArray     enable one or more package catalogers
  -d, --dockerfile string          include dockerfile for upload to Anchore Enterprise
      --exclude stringArray        exclude paths from being scanned using a glob expression
      --file string                file to write the default report output to (default is STDOUT)
  -h, --help                       help for packages
  -H, --host string                the hostname or URL of the Anchore Enterprise instance to upload to
      --import-timeout uint        set a timeout duration (in seconds) for the upload to Anchore Enterprise (default 30)
  -o, --output stringArray         report output format, options=[syft-json cyclonedx-xml cyclonedx-json github github-json spdx-tag-value spdx-json table text template] (default [table])
      --overwrite-existing-image   overwrite an existing image during the upload to Anchore Enterprise
  -p, --password string            the password to authenticate against Anchore Enterprise
      --platform string            an optional platform specifier for container image sources (e.g. 'linux/arm64', 'linux/arm64/v8', 'arm64', 'linux')
  -s, --scope string               selection of layers to catalog, options=[Squashed AllLayers] (default "Squashed")
  -t, --template string            specify the path to a Go template file
  -u, --username string            the username to authenticate against Anchore Enterprise
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -c, --config string         application config file
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
  -q, --quiet                 suppress all logging output
  -v, --verbose count         increase verbosity (-v = info, -vv = debug)
```

### SEE ALSO

* [zarf tools sbom](zarf_tools_sbom.md)	 - SBOM tools provided by Anchore Syft

