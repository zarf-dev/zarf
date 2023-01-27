# zarf tools get-creds

Display a Table of credentials for deployed components. Pass a component name to get a single credential.

## Synopsis

Display a Table of credentials for deployed components. Pass a component name to get a single credential. i.e. `zarf tools get-creds registry`

``` bash
zarf tools get-creds [flags]
```

## Options

``` bash
  -h, --help   help for get-creds
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

* [zarf tools](zarf_tools.md) - Collection of additional tools to make airgap easier
