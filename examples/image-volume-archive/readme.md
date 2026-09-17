# Image Volume Archive

Shows how to turn a plain directory of files into an OCI image and ship it
inside a Zarf package as an `imageArchive`, so a cluster can mount it as an
[image volume](https://kubernetes.io/docs/concepts/storage/volumes/#image)
without the files ever passing through a registry.

The `onCreate` action calls `zarf dev image-volume-archive` (alias `iva`):

```bash
zarf dev iva <DIRECTORY> <IMAGE-REFERENCE>
```

It walks `<DIRECTORY>`, writes each file into an OCI layer, and exports the
result as a `docker save`-compatible tar. With no `--output` the tar name is
derived from the reference, which is why `imageArchives[0].path` below matches
`zarf.internal_zarf-docs-local.tar`.

Useful flags:

- `--layer-compression` — `gzip` (default), `zstd`, or `uncompressed`.
- `--max-layers` — cap the layer count; files are batched across fewer layers
  to stay under it. `0` disables the cap.
- `--platform-os` — `linux` (default) or `windows`.
- `--output` / `-o` — write the tar somewhere other than the derived name.

Timestamps and file ownership are pinned to fixed values, so the same
directory always produces the same image digest regardless of who builds it
or when. Symlinks are stored as symlinks and never followed; devices,
sockets, and FIFOs are skipped.

:::caution

`zarf dev` commands are for package authors building packages, not for
deploy-time use.

:::
