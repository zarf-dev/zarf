# zarf tools registry
<!-- Auto-generated by hack/gen-cli-docs.sh -->

Tools for working with container registries using go-containertools

## Options

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
  -h, --help                               help for registry
      --insecure                           Allow image references to be fetched without TLS
      --platform string                    Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default "all")
  -v, --verbose                            Enable debug logs
```

## SEE ALSO

* [zarf tools](zarf_tools.md)	 - Collection of additional tools to make airgap easier
* [zarf tools registry catalog](zarf_tools_registry_catalog.md)	 - List the repos in a registry
* [zarf tools registry copy](zarf_tools_registry_copy.md)	 - Efficiently copy a remote image from src to dst while retaining the digest value
* [zarf tools registry delete](zarf_tools_registry_delete.md)	 - Delete an image reference from its registry
* [zarf tools registry digest](zarf_tools_registry_digest.md)	 - Get the digest of an image
* [zarf tools registry login](zarf_tools_registry_login.md)	 - Log in to a registry
* [zarf tools registry ls](zarf_tools_registry_ls.md)	 - List the tags in a repo
* [zarf tools registry prune](zarf_tools_registry_prune.md)	 - Prunes images from the registry that are not currently being used by any Zarf packages.
* [zarf tools registry pull](zarf_tools_registry_pull.md)	 - Pull remote images by reference and store their contents locally
* [zarf tools registry push](zarf_tools_registry_push.md)	 - Push local image contents to a remote registry