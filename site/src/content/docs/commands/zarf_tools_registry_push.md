---
title: zarf tools registry push
description: Zarf CLI command reference for <code>zarf tools registry push</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

## zarf tools registry push

Push local image contents to a remote registry

### Synopsis

If the PATH is a directory, it will be read as an OCI image layout. Otherwise, PATH is assumed to be a docker-style tarball.

```
zarf tools registry push PATH IMAGE [flags]
```

### Examples

```

# Push an image into an internal repo in Zarf
$ zarf tools registry push image.tar 127.0.0.1:31999/stefanprodan/podinfo:6.4.0

# Push an image into an repo hosted at reg.example.com
$ zarf tools registry push image.tar reg.example.com/stefanprodan/podinfo:6.4.0

```

### Options

```
  -h, --help                help for push
      --image-refs string   path to file where a list of the published image references will be written
      --index               push a collection of images as a single index, currently required if PATH contains multiple images
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --insecure-skip-tls-verify           Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --plain-http                         Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --platform string                    Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default "all")
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [zarf tools registry](/commands/zarf_tools_registry/)	 - Tools for working with container registries using go-containertools

