---
title: zarf tools sbom scan
---

## zarf tools sbom scan

Generate an SBOM

### Synopsis

Generate a packaged-based Software Bill Of Materials (SBOM) from container images and filesystems

```
zarf tools sbom scan [SOURCE] [flags]
```

### Options

```
      --base-path string         base directory for scanning, no links will be followed above this directory, and all paths will be reported relative to this directory
      --catalogers stringArray   enable one or more package catalogers
      --exclude stringArray      exclude paths from being scanned using a glob expression
      --file string              file to write the default report output to (default is STDOUT) (DEPRECATED: use: output)
  -h, --help                     help for scan
      --name string              set the name of the target being analyzed (DEPRECATED: use: source-name)
  -o, --output stringArray       report output format (<format>=<file> to output to a file), formats=[cyclonedx-json cyclonedx-xml github-json spdx-json spdx-tag-value syft-json syft-table syft-text template] (default [syft-table])
      --platform string          an optional platform specifier for container image sources (e.g. 'linux/arm64', 'linux/arm64/v8', 'arm64', 'linux')
  -s, --scope string             selection of layers to catalog, options=[squashed all-layers]
      --source-name string       set the name of the target being analyzed
      --source-version string    set the version of the target being analyzed
  -t, --template string          specify the path to a Go template file
```

### Options inherited from parent commands

```
  -c, --config string   syft configuration file
  -q, --quiet           suppress all logging output
  -v, --verbose count   increase verbosity (-v = info, -vv = debug)
```

### SEE ALSO

* [zarf tools sbom](/cli/commands/zarf_tools_sbom/)	 - Generates a Software Bill of Materials (SBOM) for the given package