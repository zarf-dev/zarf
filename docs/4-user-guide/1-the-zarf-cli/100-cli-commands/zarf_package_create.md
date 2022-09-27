## zarf package create

Use to create a Zarf package from a given directory or the current directory

### Synopsis

Builds an archive of resources and dependencies defined by the 'zarf.yaml' in the active directory.
Private registries and repositories are accessed via credentials in your local '~/.docker/config.json' and '~/.git-credentials'.


```
zarf package create [DIRECTORY] [flags]
```

### Options

```
      --confirm                   Confirm package creation without prompting
  -h, --help                      help for create
      --insecure                  Allow insecure registry connections when pulling OCI images
  -o, --output-directory string   Specify the output directory for the created Zarf package
      --set stringToString        Specify package variables to set on the command line (KEY=value) (default [])
      --skip-sbom                 Skip generating SBOM for this package
      --tmpdir string             Specify the temporary directory to use for intermediate files
      --zarf-cache string         Specify the location of the Zarf  artifact cache (images and git repositories) (default ".zarf-cache")
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
```

### SEE ALSO

* [zarf package](zarf_package.md)	 - Zarf package commands for creating, deploying, and inspecting packages

