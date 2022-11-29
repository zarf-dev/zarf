## zarf tools registry push

Push local image contents to a remote registry

### Synopsis

If the PATH is a directory, it will be read as an OCI image layout. Otherwise, PATH is assumed to be a docker-style tarball.

```
zarf tools registry push PATH IMAGE [flags]
```

### Options

```
  -h, --help                help for push
      --image-refs string   path to file where a list of the published image references will be written
      --index               push a collection of images as a single index, currently required if PATH contains multiple images
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

