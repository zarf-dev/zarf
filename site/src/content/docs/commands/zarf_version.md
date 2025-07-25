---
title: zarf version
description: Zarf CLI command reference for <code>zarf version</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

## zarf version

Shows the version of the running Zarf binary

### Synopsis

Displays the version of the Zarf release that the current binary was built from.

```
zarf version [flags]
```

### Options

```
  -h, --help                         help for version
  -o, --output-format outputFormat   Output format (yaml|json)
```

### Options inherited from parent commands

```
  -a, --architecture string        Architecture for OCI images and Zarf packages
      --insecure-skip-tls-verify   Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --log-format string          Select a logging format. Defaults to 'console'. Valid options are: 'console', 'json', 'dev'. (default "console")
  -l, --log-level string           Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-color                   Disable terminal color codes in logging and stdout prints.
      --plain-http                 Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --tmpdir string              Specify the temporary directory to use for intermediate files
      --zarf-cache string          Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf](/commands/zarf/)	 - The Airgap Native Packager Manager for Kubernetes

