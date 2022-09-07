## zarf tools sbom convert

Convert between SBOM formats

### Synopsis

[Experimental] Convert SBOM files to, and from, SPDX, CycloneDX and Syft's format. For more info about data loss between formats see https://github.com/anchore/syft#format-conversion-experimental

```
zarf tools sbom convert [SOURCE-SBOM] -o [FORMAT] [flags]
```

### Examples

```
  syft convert img.syft.json -o spdx-json                      convert a syft SBOM to spdx-json, output goes to stdout in table format, by default
  syft convert img.syft.json -o cyclonedx-json=img.cdx.json    convert a syft SBOM to CycloneDX, output goes to a file named img.cdx.json

```

### Options

```
  -h, --help   help for convert
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

