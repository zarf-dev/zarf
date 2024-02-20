---
title: zarf tools sbom
---

## zarf tools sbom

Generates a Software Bill of Materials (SBOM) for the given package

### Synopsis

Generate a packaged-based Software Bill Of Materials (SBOM) from container images and filesystems

```
zarf tools sbom [flags]
```

### Options

```
      --base-path string         base directory for scanning, no links will be followed above this directory, and all paths will be reported relative to this directory
      --catalogers stringArray   enable one or more package catalogers
  -c, --config string            syft configuration file
      --exclude stringArray      exclude paths from being scanned using a glob expression
      --file string              file to write the default report output to (default is STDOUT) (DEPRECATED: use: output)
  -h, --help                     help for sbom
      --name string              set the name of the target being analyzed (DEPRECATED: use: source-name)
  -o, --output stringArray       report output format (<format>=<file> to output to a file), formats=[cyclonedx-json cyclonedx-xml github-json spdx-json spdx-tag-value syft-json syft-table syft-text template] (default [syft-table])
      --platform string          an optional platform specifier for container image sources (e.g. 'linux/arm64', 'linux/arm64/v8', 'arm64', 'linux')
  -q, --quiet                    suppress all logging output
  -s, --scope string             selection of layers to catalog, options=[squashed all-layers] (default "squashed")
      --source-name string       set the name of the target being analyzed
      --source-version string    set the version of the target being analyzed
  -t, --template string          specify the path to a Go template file
  -v, --verbose count            increase verbosity (-v = info, -vv = debug)
```

### SEE ALSO

* [zarf tools](/cli/commands/zarf_tools/)	 - Collection of additional tools to make airgap easier
* [zarf tools sbom attest](/cli/commands/zarf_tools_sbom_attest/)	 - Generate an SBOM as an attestation for the given [SOURCE] container image
* [zarf tools sbom convert](/cli/commands/zarf_tools_sbom_convert/)	 - Convert between SBOM formats
* [zarf tools sbom login](/cli/commands/zarf_tools_sbom_login/)	 - Log in to a registry
* [zarf tools sbom scan](/cli/commands/zarf_tools_sbom_scan/)	 - Generate an SBOM
* [zarf tools sbom version](/cli/commands/zarf_tools_sbom_version/)	 - show version information
