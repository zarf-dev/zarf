# zarf tools registry catalog

List the repositories in a registry

``` bash
zarf tools registry catalog [REGISTRY] [flags]
```

## Examples

``` bash
  # list the repositories internal to Zarf
  $ zarf tools registry catalog

  # list the repositories for reg.example.com
  $ zarf tools registry catalog reg.example.com
```

## Options

``` bash
  -h, --help   help for catalog
```

## Options inherited from parent commands

``` bash
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-log-file           Disable log file creation
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

## SEE ALSO

* [zarf tools registry](zarf_tools_registry.md) - Tools for working with container registries using `go-containertools`.
