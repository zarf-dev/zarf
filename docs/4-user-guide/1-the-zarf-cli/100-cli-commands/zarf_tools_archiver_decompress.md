## zarf tools archiver decompress

Decompress an archive or Zarf package based off of the source file extension.

```
zarf tools archiver decompress {ARCHIVE} {DESTINATION} [flags]
```

### Options

```
  -h, --help   help for decompress
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

* [zarf tools archiver](zarf_tools_archiver.md)	 - Compress/Decompress generic archives, including Zar packages.

