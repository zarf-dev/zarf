# Image Volume Archive

Shows how to turn a plain directory of files into an OCI image and ship it inside a Zarf package as an `imageArchive`, so a cluster can mount it as an [image volume](https://kubernetes.io/docs/concepts/storage/volumes/#image) without the files ever passing through a registry.

The `onCreate` action calls `zarf dev image-volume-archive` (alias `iva`):

```bash
zarf dev iva <DIRECTORY> <IMAGE-REFERENCE>
```

It walks `<DIRECTORY>`, writes each file into an OCI layer, and exports the result as a `docker save`-compatible tar. With no `--output` the tar name is derived from the reference, which is why `imageArchives[0].path` below matches `zarf.internal_zarf-docs-local.tar`.

Useful flags:

- `--layer-compression` — `gzip` (default), `zstd`, or `uncompressed`.
- `--max-layers` — cap the layer count; files are batched across fewer layers   to stay under it. `0` disables the cap.
- `--platform-os` — `linux` (default) or `windows`.
- `--output` / `-o` — write the tar somewhere other than the derived name.

Timestamps and file ownership are pinned to fixed values, so the same directory always produces the same image digest regardless of who builds it or when. Symlinks are stored as symlinks and never followed; devices, sockets, and FIFOs are skipped.

:::caution

`zarf dev` commands are for package authors building packages, not for deploy-time use.

:::

## The two components

`docs-archive` packs this documentation site's built output and serves it from an **image volume**: nginx mounts the archive read-only at its document root, so the HTML is delivered straight out of the OCI image with no copy step, no `PersistentVolumeClaim`, and no init container. The site is built first by `site/hack/build-in-docker.sh`, which runs the same Astro build Netlify runs inside a container pinned to `netlify.toml`'s `NODE_VERSION` — no local Node install required.

`fluxcd-oci-repo` packs a directory of Flux-ready manifests and lets a Flux `OCIRepository` **extract** a layer from it, treating the contents as a source of truth for a `Kustomization`.

The difference matters for how you build the archive:

|                     | `docs-archive` (image volume)                    | `fluxcd-oci-repo` (Flux source)                          |
| ------------------- | ------------------------------------------------ | -------------------------------------------------------- |
| Consumer            | kubelet, via `volumes[].image`                   | Flux source-controller                                   |
| Layers              | all layers flattened, so the default cap is fine | **one** — Flux reads a single layer, so `--max-layers 1` |
| Cluster requirement | OCI volume support (GA in Kubernetes 1.35)       | none beyond Flux                                         |

## Serving the archive with nginx

Three things in `nginx/` are what make a static site work off a read-only volume:

- **`absolute_redirect off`** — Astro uses directory-style URLs, so a request for `/ref/create` gets a 301 to `/ref/create/`. nginx would normally send an absolute `Location` that drops the port, which breaks `zarf connect`'s arbitrary local port. Relative redirects stay correct behind any   port-forward or proxy.
- **`try_files $uri $uri/ =404`** — resolves `/ref/create/` to the `ref/create/index.html` inside the volume.
- **`error_page 404 /404.html`** — serves Astro's generated 404 page instead of nginx's built-in one.

The nginx config comes from a `ConfigMap` rather than the archive, because the image volume is mounted read-only and nginx's own config lives at a different path. Only the site content travels in the archive.

Deploy it and open the site with:

```bash
zarf connect docs
```
