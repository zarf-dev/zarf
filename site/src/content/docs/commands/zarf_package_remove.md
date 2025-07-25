---
title: zarf package remove
description: Zarf CLI command reference for <code>zarf package remove</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

## zarf package remove

Removes a Zarf package that has been deployed already (runs offline)

### Synopsis

Removes a Zarf package that has been deployed already (runs offline). Remove reverses the deployment order, the last component is removed first.

```
zarf package remove { PACKAGE_SOURCE | PACKAGE_NAME } --confirm [flags]
```

### Options

```
      --components string           Comma-separated list of components to remove.  This list will be respected regardless of a component's 'required' or 'default' status.  Globbing component names with '*' and deselecting components with a leading '-' are also supported.
      --confirm                     REQUIRED. Confirm the removal action to prevent accidental deletions
  -h, --help                        help for remove
  -n, --namespace string            [Alpha] Override the namespace for package removal. Applicable only to packages deployed using the namespace flag.
      --skip-signature-validation   Skip validating the signature of the Zarf package
```

### Options inherited from parent commands

```
  -a, --architecture string        Architecture for OCI images and Zarf packages
      --insecure-skip-tls-verify   Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -k, --key string                 Path to public key file for validating signed packages
      --log-format string          Select a logging format. Defaults to 'console'. Valid options are: 'console', 'json', 'dev'. (default "console")
  -l, --log-level string           Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-color                   Disable terminal color codes in logging and stdout prints.
      --oci-concurrency int        Number of concurrent layer operations when pulling or pushing images or packages to/from OCI registries. (default 6)
      --plain-http                 Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --tmpdir string              Specify the temporary directory to use for intermediate files
      --zarf-cache string          Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf package](/commands/zarf_package/)	 - Zarf package commands for creating, deploying, and inspecting packages

