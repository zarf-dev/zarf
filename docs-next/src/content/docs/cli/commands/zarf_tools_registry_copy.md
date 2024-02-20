---
title: zarf tools registry copy
---

## zarf tools registry copy

Efficiently copy a remote image from src to dst while retaining the digest value

```
zarf tools registry copy SRC DST [flags]
```

### Options

```
  -a, --all-tags     (Optional) if true, copy all tags from SRC to DST
  -h, --help         help for copy
  -j, --jobs int     (Optional) The maximum number of concurrent copies, defaults to GOMAXPROCS
  -n, --no-clobber   (Optional) if true, avoid overwriting existing tags in DST
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --platform string                    Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default "all")
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [zarf tools registry](/cli/commands/zarf_tools_registry/)	 - Tools for working with container registries using go-containertools