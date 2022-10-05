## zarf prepare generate-config

Generates a config file for Zarf

### Synopsis

Generates a Zarf config file for controlling how the Zarf CLI operates. Optionally accepts a filename to write the config to.

The extension will determine the format of the config file, e.g. env-1.yaml, env-2.json, env-3.toml etc. 
Accepted extensions are json, toml, yaml.

NOTE: This file must not already exist. If no filename is provided, the config will be written to the current working directory as zarf-config.toml.

```
zarf prepare generate-config [FILENAME] [flags]
```

### Options

```
  -h, --help   help for generate-config
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

* [zarf prepare](zarf_prepare.md)	 - Tools to help prepare assets for packaging

