# zarf completion fish

Generate the autocompletion script for fish

## Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

``` bash
zarf completion fish | source
```

To load completions for every new session, execute once:

``` bash
zarf completion fish > ~/.config/fish/completions/zarf.fish
```

You will need to start a new shell for this setup to take effect.

``` bash
zarf completion fish [flags]
```

## Options

``` bash
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
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

* [zarf completion](zarf_completion.md)  - Generate the autocompletion script for the specified shell
