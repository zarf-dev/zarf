---
title: zarf dev inspect definition
description: Zarf CLI command reference for <code>zarf dev inspect definition</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

## zarf dev inspect definition

Displays the fully rendered package definition

### Synopsis

Displays the 'zarf.yaml' definition of a Zarf after package templating, flavors, and component imports are applied

```
zarf dev inspect definition [ DIRECTORY ] [flags]
```

### Options

```
  -f, --flavor string        The flavor of components to include in the resulting package (i.e. have a matching or empty "only.flavor" key)
  -h, --help                 help for definition
      --set stringToString   Specify package variables to set on the command line (KEY=value) (default [])
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

* [zarf dev inspect](/commands/zarf_dev_inspect/)	 - Commands to get information about a Zarf package using a `zarf.yaml`

