# zarf prepare find-images

Evaluates components in a zarf file to identify images specified in their helm charts and manifests

## Synopsis

Evaluates components in a zarf file to identify images specified in their helm charts and manifests.

Components that have repositories that host helm charts can be processed by providing the `--repo-chart-path`.

``` bash
zarf prepare find-images [PACKAGE] [flags]
```

## Options

``` bash
  -h, --help                     help for find-images
  -p, --repo-chart-path string   If git repositories hold helm charts, often found with gitops tools, specify the chart path, e.g. "/" or "/chart"
      --set stringToString       Specify package variables to set on the command line (KEY=value). Note, if using a config file, this will be set by [package.create.set]. (default [])
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

* [zarf prepare](zarf_prepare.md) - Tools to help prepare assets for packaging
