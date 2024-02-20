---
title: zarf tools registry
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

### SEE ALSO

* [zarf tools](/cli/commands/zarf_tools/)	 - Collection of additional tools to make airgap easier
* [zarf tools registry catalog](/cli/commands/zarf_tools_registry_catalog/)	 - List the repos in a registry
* [zarf tools registry copy](/cli/commands/zarf_tools_registry_copy/)	 - Efficiently copy a remote image from src to dst while retaining the digest value
* [zarf tools registry delete](/cli/commands/zarf_tools_registry_delete/)	 - Delete an image reference from its registry
* [zarf tools registry digest](/cli/commands/zarf_tools_registry_digest/)	 - Get the digest of an image
* [zarf tools registry login](/cli/commands/zarf_tools_registry_login/)	 - Log in to a registry
* [zarf tools registry ls](/cli/commands/zarf_tools_registry_ls/)	 - List the tags in a repo
* [zarf tools registry prune](/cli/commands/zarf_tools_registry_prune/)	 - Prunes images from the registry that are not currently being used by any Zarf packages.
* [zarf tools registry pull](/cli/commands/zarf_tools_registry_pull/)	 - Pull remote images by reference and store their contents locally
* [zarf tools registry push](/cli/commands/zarf_tools_registry_push/)	 - Push local image contents to a remote registry