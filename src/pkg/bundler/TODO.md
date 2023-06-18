- [ ] re-signing packages?
- [ ] keychains?
- [ ] fs?

```text
structure of a bundle tarball:

since bundles are essentially collections of OCI images (Zarf packages that have been published as OCI images), we can use the OCI image format to store the bundle.

zarf-bundle.tar.zst
├── zarf-bundle.yaml # basically an enhanced OCI index.json
├── zarf-bundle.sig
├── index.json
├── oci-layout
├── blobs/sha256
│   └── ... (all blobs from all the packages)
```
