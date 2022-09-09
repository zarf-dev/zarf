## zarf tools sbom packages

Generate a package SBOM

### Synopsis

Generate a packaged-based Software Bill Of Materials (SBOM) from container images and filesystems

```
zarf tools sbom packages [SOURCE] [flags]
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

