# Remote Component Import Plan

## Goal

Remote component imports should behave like local component imports after load: the package definition sees normal local-looking paths, while transport details stay in transient loader/assembly state.

This plan assumes implementation happens after package assembly has native `v1beta1` support. Until then, threading remote resource descriptors through the current `AsV1alpha1()` assembly path would add churn without improving the final design.

## Design decisions

1. Do not put a full resource table in `ComponentConfig.publishData`.
   - `publishData` remains generated producer metadata: Zarf version, migrations, and version requirements.
   - Per-resource OCI descriptors are transport metadata and should not become user-authored schema surface area.
2. Use the OCI manifest as the persistent resource index.
   - Component publish records logical resource mount paths on OCI layer descriptors via annotations.
   - Component import reconstructs the resource index from the manifest during load.
3. Carry remote resource descriptors on `load.ResolvedPackage`.
   - `ResolvedPackage` already carries transient import state with `ImportedSchemas`.
   - Remote resources are the same category: derived during load, consumed during assembly, not part of the user package schema.
4. Rebase imported remote paths to a logical import root.
   - Local imports already rebase child paths so the merged package has paths relative to the importing package.
   - Remote imports should do the same, using a synthetic root such as `.zarf/imports/<stable-id>/`.
5. Assembly hydrates remote resources.
   - Loading resolves metadata and paths.
   - Assembly pulls/extracts the required remote blobs into its operation workspace and resolves the synthetic paths to real local files.
6. `zarf dev inspect definition` shows logical imported paths.
   - It should not leak `~/.zarf-cache` paths.
   - It can show `.zarf/imports/<stable-id>/...` even when the resources are not materialized until assembly.

## Current behavior to preserve

Local v1beta1 imports follow this semantic rule:

```text
all relative resource paths in the resolved package are relative to the importing package's zarf.yaml directory
```

Example source layout:

```text
pkg/
  zarf.yaml
  components/
    logging/
      component.yaml
      files/config.yaml
```

`pkg/zarf.yaml`:

```yaml
components:
  - name: logging
    import:
      local:
        - path: components/logging/component.yaml
```

`components/logging/component.yaml`:

```yaml
component:
  files:
    - source: files/config.yaml
      destination: /etc/config.yaml
```

Resolved package:

```yaml
components:
  - name: logging
    files:
      - source: components/logging/files/config.yaml
        destination: /etc/config.yaml
```

Remote imports should preserve the same invariant. After import resolution, the merged package should not contain paths whose base directory is ambiguous.

## Target path model

A remote component has no directory inside the importing package, so import resolution creates a logical import root:

```text
.zarf/imports/<stable-import-id>/
```

`<stable-import-id>` should be derived from the pulled component artifact manifest digest, not from a mutable tag. A filesystem-safe form is acceptable:

```text
.zarf/imports/sha256-abcdef.../
```

Remote component source:

```yaml
apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: logging
  version: 1.0.0
component:
  files:
    - source: files/config.yaml
      destination: /etc/config.yaml
  charts:
    - name: app
      local:
        path: charts/app
  manifests:
    - name: raw
      files:
        - manifests/configmap.yaml
```

Importing package:

```yaml
components:
  - name: logging
    import:
      remote:
        - url: oci://registry.example/team/logging:1.0.0
```

Resolved package shown by `zarf dev inspect definition`:

```yaml
components:
  - name: logging
    files:
      - source: .zarf/imports/sha256-abcdef/files/config.yaml
        destination: /etc/config.yaml
    charts:
      - name: app
        local:
          path: .zarf/imports/sha256-abcdef/charts/app
    manifests:
      - name: raw
        files:
          - .zarf/imports/sha256-abcdef/manifests/configmap.yaml
```

These are logical paths relative to the importing package root. Assembly later materializes them.

## Why raw imported paths should not be preserved

If the imported path stays as:

```yaml
source: files/config.yaml
```

then after merge it appears to mean:

```text
<importing-package>/files/config.yaml
```

But the intended source is:

```text
<remote-component-root>/files/config.yaml
```

That ambiguity matters when both the importing package and the remote component contain the same relative path. The current API structs carry plain strings; once components are merged, resource provenance is otherwise lost.

Keeping raw paths would require a larger internal model where every resource path has an explicit origin root. The synthetic import root gives the same behavior while preserving the existing string-path contract.

## OCI artifact layout

Component publish should store resources as OCI layers/blobs and annotate each descriptor with the logical mount path inside the component root.

Example manifest layers:

```text
component.yaml layer or config blob
  mediaType: application/vnd.zarf.component.config.v1+yaml

files layer
  mediaType: application/vnd.zarf.resource.directory.v1.tar
  digest: sha256:...
  annotations:
    dev.zarf.mountPath: files
    dev.zarf.resourceKind: files

chart layer
  mediaType: application/vnd.zarf.resource.directory.v1.tar
  digest: sha256:...
  annotations:
    dev.zarf.mountPath: charts/app
    dev.zarf.resourceKind: chart

manifest layer
  mediaType: application/vnd.zarf.resource.directory.v1.tar
  digest: sha256:...
  annotations:
    dev.zarf.mountPath: manifests
    dev.zarf.resourceKind: manifests
```

Most Zarf resources can remain tar files. The remote resource index does not need one descriptor per file if a directory tar covers the path.

Path resolution should use longest-prefix matching:

```text
.zarf/imports/sha256-abcdef/files/config.yaml
  import root: .zarf/imports/sha256-abcdef
  relative path: files/config.yaml
  matching mount: files
  descriptor: files tar layer
  hydrated path: <assembly-workdir>/imports/sha256-abcdef/files/config.yaml
```

## Data model

Add transient import state to `load.ResolvedPackage` after native v1beta1 assembly exists.

Possible shape:

```go
type ResolvedPackage struct {
    PackageDefinition api.PackageDefinition
    ImportedSchemas   []string
    RemoteResources   []RemoteResource
}

type RemoteResource struct {
    ImportRoot string
    MountPath  string
    Kind       RemoteResourceKind
    Descriptor ocispec.Descriptor
    Repository registry.Reference
    Archive    *RemoteResourceArchive
}

type RemoteResourceArchive struct {
    Format string
}
```

`ImportRoot` is the logical path prefix used in the resolved package definition:

```text
.zarf/imports/sha256-abcdef
```

`MountPath` is the path inside the remote component root that the descriptor materializes:

```text
files
charts/app
manifests
values
```

`Descriptor` carries OCI-native identity:

```text
mediaType
digest
size
annotations
```

The resource list is generated by the loader from the pulled OCI manifest. It is not parsed from user-authored YAML.

## Publish flow

`zarf component publish COMPONENT_FILE OCI_REPOSITORY`:

1. Read the v1beta1 `ZarfComponentConfig`.
2. Validate the component config and component-equivalent package behavior.
3. Collect local resources referenced by the component.
4. Package directory resources as tar blobs where appropriate.
5. Push or stage each resource as an OCI layer/blob.
6. Annotate each resource descriptor with:
   - `dev.zarf.mountPath`
   - `dev.zarf.resourceKind`
   - optional future metadata needed by assembly
7. Write generated component config with minimal `publishData`.
8. Push the component artifact manifest.
9. Print the final `oci://` reference.

## Load/import flow

When resolving a v1beta1 remote component import:

1. Parse the remote OCI ref.
2. Pull the component artifact manifest and component config.
3. Compute the stable import root from the manifest digest:

   ```text
   .zarf/imports/sha256-abcdef
   ```

4. Reconstruct `[]RemoteResource` from annotated manifest descriptors.
5. Select the compatible imported component by architecture and flavor.
6. Resolve nested imports recursively.
7. Rebase all imported component resource paths to the import root.
8. Merge the imported component spec into the importing package component.
9. Clear the `import` field from the resolved component.
10. Return:

    ```go
    ResolvedPackage{
        PackageDefinition: resolvedDefinition,
        ImportedSchemas: importedSchemas,
        RemoteResources: remoteResources,
    }
    ```

Nested remote imports append their own `RemoteResource` entries using their own manifest-derived import roots.

## Assembly flow

Native v1beta1 assembly should receive the full `ResolvedPackage`.

Before processing local file/chart/manifest/image archive paths, assembly builds a resolver from `RemoteResources`:

```text
logical import root + mount path -> OCI descriptor
```

When assembly encounters a path:

```text
.zarf/imports/sha256-abcdef/files/config.yaml
```

it:

1. Finds the matching `RemoteResource` by longest-prefix match.
2. Pulls the descriptor from the source repository if not already hydrated.
3. Verifies the digest.
4. Extracts archives into the assembly workspace.
5. Returns the real local path to the existing assembly logic.

Existing local package paths remain unchanged and bypass the remote resolver.

## Validation behavior

Validation should distinguish three path classes:

1. URLs: unchanged existing behavior.
2. Normal local relative paths: validate relative to the package root.
3. Remote logical import paths: validate that a matching `RemoteResource` exists.

Validation should not require `.zarf/imports/<id>/...` to exist on disk before assembly.

## `zarf dev inspect definition`

Inspect should display the resolved package with logical remote import paths:

```yaml
source: .zarf/imports/sha256-abcdef/files/config.yaml
```

It should not pull and extract every resource just to print the definition unless a future flag explicitly requests materialization.

This output is intentionally logical, not a guarantee that the path exists on disk after the command exits.

## Cache and lifecycle

The synthetic path shown in the resolved definition is not required to be the physical cache path.

Recommended lifecycle:

```text
loader:
  builds descriptors only

assembly:
  hydrates descriptors into an operation workspace

cache:
  optionally stores blobs by digest for reuse
```

A later optimization can cache by descriptor digest:

```text
~/.zarf-cache/oci-blobs/sha256/<digest>
```

The resolved package should still show the logical `.zarf/imports/<id>/...` path, not the cache path.

## Error handling

Remote import resolution should fail early when:

- the OCI ref is invalid;
- the component config is missing;
- the component config is not `apiVersion: zarf.dev/v1beta1`;
- the component config is not `kind: ZarfComponentConfig`;
- there is no compatible component variant;
- there are multiple compatible component variants;
- an annotated resource descriptor has an invalid or duplicate mount path;
- a component resource path has no matching remote descriptor;
- a remote import cycle is detected.

Assembly should fail when:

- pulling a descriptor fails;
- digest verification fails;
- archive extraction fails;
- a hydrated path escapes the import root;
- a hydrated file expected by the component does not exist.

## Security requirements

- Use manifest digest, not tag, for the stable import root after the manifest is fetched.
- Verify every pulled resource descriptor digest.
- Reject archive entries that escape the target hydration directory.
- Reject absolute mount paths and mount paths containing `..` traversal.
- Treat descriptor annotations as untrusted registry data until validated.
- Keep registry credentials scoped through existing remote options.

## Testing plan

Unit tests for load/import:

- remote import rebases file paths to `.zarf/imports/<digest>/...`;
- remote import rebases chart local paths;
- remote import rebases manifest files and kustomize paths;
- imported values files and schemas keep precedence rules;
- local and remote variants select exactly one compatible component;
- nested remote imports get distinct import roots;
- remote cycles fail using canonical refs;
- missing resource descriptors fail clearly;
- duplicate mount paths fail clearly;
- invalid mount path traversal fails clearly.

Unit tests for assembly resolver:

- longest-prefix mount matching;
- tar archive hydration;
- digest verification failure;
- archive traversal rejection;
- local paths bypass the remote resolver;
- repeated paths reuse an already hydrated descriptor.

E2E test after native v1beta1 assembly:

1. Create a fixture remote component containing:
   - file resource;
   - chart directory;
   - chart values file;
   - manifest file;
   - kustomize directory;
   - values file and schema.
2. Publish it to the in-memory OCI registry used by the e2e suite.
3. Create/import a v1beta1 package that references it with `import.remote`.
4. Run `zarf dev inspect definition` and assert logical `.zarf/imports/<digest>/...` paths appear.
5. Run package create after v1beta1 assembly support exists.
6. Inspect the package layout and verify expected resources were included.

## Implementation phases

### Phase 1: Artifact descriptor annotations

- Add constants for Zarf OCI annotation keys and media types.
- Update component publish to annotate resource layers with logical mount paths.
- Validate mount paths before push.
- Keep `ComponentConfig.publishData` minimal.

### Phase 2: Remote resource index during load

- Extend remote component pull/load code to return manifest metadata and descriptors.
- Build `RemoteResource` entries from OCI descriptor annotations.
- Derive `ImportRoot` from the fetched manifest digest.
- Rebase remote component paths to `ImportRoot`.
- Return remote resources on `ResolvedPackage`.

### Phase 3: Native v1beta1 assembly integration

- Add a remote resource resolver to assembly.
- Hydrate matching resources into the assembly workspace.
- Replace logical import paths with hydrated local paths at the assembly boundary.
- Keep existing local path handling unchanged.

### Phase 4: Inspect and validation polish

- Ensure `zarf dev inspect definition` prints logical paths.
- Teach validation to accept logical remote import paths only when backed by a `RemoteResource`.
- Improve errors for missing descriptors and bad mount paths.

### Phase 5: Tests and compatibility

- Add focused unit coverage for loader, resolver, and archive safety.
- Add the v1beta1 remote import e2e after v1beta1 assembly exists.
- Verify older component publish/import behavior remains unchanged where still supported.

## Open decisions

1. Exact annotation keys.

   Proposed:

   ```text
   dev.zarf.mountPath
   dev.zarf.resourceKind
   ```

2. Exact synthetic import root prefix.

   Proposed:

   ```text
   .zarf/imports/sha256-<manifest-digest>
   ```

3. Whether `component.yaml` is an OCI config blob or a normal layer.

   The import design works either way as long as the loader can fetch it before assembly.

4. Whether to hydrate on `dev inspect definition` behind an optional flag.

   Default should be no hydration; print logical paths only.

5. Whether to cache hydrated archives or only raw blobs.

   Raw blob cache by digest is safer and easier to invalidate. Hydrated directory cache can be added later if performance requires it.

## Summary

Remote component import should become:

```text
OCI artifact manifest
  -> annotated resource descriptors
  -> load reconstructs RemoteResources
  -> paths rebase to .zarf/imports/<manifest-digest>/...
  -> ResolvedPackage carries descriptors
  -> assembly hydrates blobs by digest
  -> existing package resources are assembled from local paths
```

This avoids storing a large resource table in `publishData`, preserves the existing local-import path invariant, keeps dev inspect output understandable, and keeps transport-specific OCI state out of the user package schema.
