---
title: zarf tools archiver
description: Zarf CLI command reference for <code>zarf tools archiver</code>.
tableOfContents: false
---

## zarf tools archiver

Compresses/Decompresses generic archives, including Zarf packages

### Options

```
  -h, --help   help for archiver
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images and Zarf packages
      --insecure              Allow access to insecure registries and disable other recommended security enforcements such as package checksum and signature validation. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-color              Disable colors in output
      --no-log-file           Disable log file creation
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf tools](/commands/zarf_tools/)	 - Collection of additional tools to make airgap easier
* [zarf tools archiver compress](/commands/zarf_tools_archiver_compress/)	 - Compresses a collection of sources based off of the destination file extension.
* [zarf tools archiver decompress](/commands/zarf_tools_archiver_decompress/)	 - Decompresses an archive or Zarf package based off of the source file extension.
* [zarf tools archiver version](/commands/zarf_tools_archiver_version/)	 - Print the version
