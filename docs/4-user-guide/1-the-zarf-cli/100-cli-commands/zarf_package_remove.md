## zarf package remove

Use to remove a Zarf package that has been deployed already

```
zarf package remove {PACKAGE_NAME} [flags]
```

### Options

```
      --components string   Comma-separated list of components to uninstall
      --confirm             REQUIRED. Confirm the removal action to prevent accidental deletions
  -h, --help                help for remove
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-log-file           Disable log file creation.
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
      --tmpdir string         Specify the temporary directory to use for intermediate files
```

### SEE ALSO

* [zarf package](zarf_package.md)	 - Zarf package commands for creating, deploying, and inspecting packages

