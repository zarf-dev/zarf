## zarf tools gen-pki

Generates a Certificate Authority and PKI chain of trust for the given host

```
zarf tools gen-pki {HOST} [flags]
```

### Options

```
  -h, --help                       help for gen-pki
      --sub-alt-name stringArray   Specify Subject Alternative Names for the certificate
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

* [zarf tools](zarf_tools.md)	 - Collection of additional tools to make airgap easier

