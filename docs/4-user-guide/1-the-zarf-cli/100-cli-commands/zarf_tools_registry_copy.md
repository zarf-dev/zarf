## zarf tools registry copy

Efficiently copy a remote image from src to dst while retaining the digest value

```
zarf tools registry copy SRC DST [flags]
```

### Options

```
  -h, --help   help for copy
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

* [zarf tools registry](zarf_tools_registry.md)	 - Tools for working with container registries using go-containertools.

