## zarf tools sbom packages

Generate a package SBOM

### Synopsis

Generate a packaged-based Software Bill Of Materials (SBOM) from container images and filesystems

```
zarf tools sbom packages [SOURCE] [flags]
```

### Options

```
      --catalogers stringArray   enable one or more package catalogers
      --exclude stringArray      exclude paths from being scanned using a glob expression
      --file string              file to write the default report output to (default is STDOUT)
  -h, --help                     help for packages
      --name string              set the name of the target being analyzed
  -o, --output stringArray       report output format, options=[syft-json cyclonedx-xml cyclonedx-json github github-json spdx-tag-value spdx-json table text template] (default [table])
      --platform string          an optional platform specifier for container image sources (e.g. 'linux/arm64', 'linux/arm64/v8', 'arm64', 'linux')
  -s, --scope string             selection of layers to catalog, options=[Squashed AllLayers] (default "Squashed")
  -t, --template string          specify the path to a Go template file
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -c, --config string         application config file
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-log-file           Disable log file creation
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
  -q, --quiet                 suppress all logging output
      --tmpdir string         Specify the temporary directory to use for intermediate files
  -v, --verbose count         increase verbosity (-v = info, -vv = debug)
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf tools sbom](zarf_tools_sbom.md)	 - Generates a Software Bill of Materials (SBOM) for the given package

