## zarf tools get-git-password

Returns the push user's password for the Git server

### Synopsis

Reads the password for a user with push access to the configured Git server from the zarf-state secret in the zarf namespace

```
zarf tools get-git-password [flags]
```

### Options

```
  -h, --help   help for get-git-password
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
      --insecure              Allow access to insecure registries and disable other recommended security enforcements. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-log-file           Disable log file creation
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf tools](zarf_tools.md)	 - Collection of additional tools to make airgap easier

