## zarf package inspect

Lists the payload of a Zarf package (runs offline)

### Synopsis

Lists the payload of a compiled package file (runs offline)
Unpacks the package tarball into a temp directory and displays the contents of the archive.

```
zarf package inspect [PACKAGE] [flags]
```

### Options

```
  -h, --help            help for inspect
  -s, --sbom            View SBOM contents while inspecting the package.
      --tmpdir string   Specify the temporary directory to use for intermediate files
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
```

### SEE ALSO

* [zarf package](zarf_package.md)	 - Zarf package commands for creating, deploying, and inspecting packages

