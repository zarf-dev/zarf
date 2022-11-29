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
  -h, --help   help for inspect
  -s, --sbom   View SBOM contents while inspecting the package
```

### Options inherited from parent commands

```
  -a, --architecture string   Set the architecture to use for the package. Valid options are: amd64, arm64.
  -l, --log-level string      Set the log level. Valid options are: warn, info, debug, trace. (default "info")
      --no-log-file           Disable logging to a file.
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf package](zarf_package.md)	 - Zarf package commands for creating, deploying, and inspecting packages

