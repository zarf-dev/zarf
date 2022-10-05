## zarf prepare patch-git

Converts all .git URLs to the specified Zarf HOST and with the Zarf URL pattern in a given FILE.  NOTE: 
This should only be used for manifests that are not mutated by the Zarf Agent Mutating Webhook.

```
zarf prepare patch-git [HOST] [FILE] [flags]
```

### Options

```
      --git-account string   User or organization name for the git account that the repos are created under. (default "zarf-git-user")
  -h, --help                 help for patch-git
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

