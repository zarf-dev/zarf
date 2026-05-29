---
title: zarf tools registry
description: Zarf CLI command reference for <code>zarf tools registry</code>.
tableOfContents: false
slug: v0.77/commands/zarf_tools_registry
---

## zarf tools registry

Tools for working with container registries using go-containertools

### Options

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
  -h, --help                               help for registry
      --insecure                           Allow image references to be fetched without TLS
      --platform string                    Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default "all")
  -v, --verbose                            Enable debug logs
```

### Options inherited from parent commands

```
      --features stringToString    Provide a comma-separated list of feature names to bools to enable or disable. Ex. --features "foo=true,bar=false,baz=true" (default [])
      --insecure-skip-tls-verify   Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --plain-http                 Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.
```

### SEE ALSO

* [zarf tools](/v0.77/commands/zarf_tools/)	 - Collection of additional tools to make airgap easier
* [zarf tools registry catalog](/v0.77/commands/zarf_tools_registry_catalog/)	 - List the repos in a registry
* [zarf tools registry copy](/v0.77/commands/zarf_tools_registry_copy/)	 - Efficiently copy a remote image from src to dst while retaining the digest value
* [zarf tools registry delete](/v0.77/commands/zarf_tools_registry_delete/)	 - Delete an image reference from its registry
* [zarf tools registry digest](/v0.77/commands/zarf_tools_registry_digest/)	 - Get the digest of an image
* [zarf tools registry export](/v0.77/commands/zarf_tools_registry_export/)	 - Export filesystem of a container image as a tarball
* [zarf tools registry login](/v0.77/commands/zarf_tools_registry_login/)	 - Login to a container registry
* [zarf tools registry ls](/v0.77/commands/zarf_tools_registry_ls/)	 - List the tags in a repo
* [zarf tools registry manifest](/v0.77/commands/zarf_tools_registry_manifest/)	 - Get the manifest of an image
* [zarf tools registry prune](/v0.77/commands/zarf_tools_registry_prune/)	 - Prunes images from the registry that are not currently being used by any Zarf packages.
* [zarf tools registry pull](/v0.77/commands/zarf_tools_registry_pull/)	 - Pull remote images by reference and store their contents locally
* [zarf tools registry push](/v0.77/commands/zarf_tools_registry_push/)	 - Push local image contents to a remote registry
* [zarf tools registry version](/v0.77/commands/zarf_tools_registry_version/)	 - Print the version
