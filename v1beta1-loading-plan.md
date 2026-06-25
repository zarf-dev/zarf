# Plan: Loading v1beta1 packages (Milestone 1 — `dev inspect definition`)

## Goal

Make `zarf dev inspect definition <dir>` work when the local `zarf.yaml` is a
v1beta1 package. Today this errors with `unsupported apiVersion "zarf.dev/v1beta1"`
because `pkgcfg.Parse` only knows v1alpha1.

This is the first slice of the larger effort to load v1beta1 packages, following
the ordering in ZEP-0048 (load → lint/find-images → create → deploy).

References: `proposals/0048-schema-update-process` and `proposals/0051-v1beta1-schema`.

## Decisions

- **`load.PackageDefinition` stays the only exported entry point** and keeps its
  current signature, returning `DefinedPackage{ Pkg v1alpha1.ZarfPackage }`.
  Nothing downstream (including `cmd/dev.go`) changes.
- **Branch internally by apiVersion.** `pkgcfg` reports the version; `PackageDefinition`
  dispatches to private `v1alpha1PackageDefinition` / `v1beta1PackageDefinition`.
  Parse, validation, and import resolution are each version-specific, so the branch
  lives at the top and each path owns its logic.
- **v1beta1 converts back to v1alpha1 at the end** of its branch via
  `convert.PackageV1Beta1ToV1Alpha1`, so the rest of Zarf keeps operating on
  v1alpha1 untouched. The codebase-wide flip to "v1beta1 as the internal currency"
  (ZEP-0048) is a later milestone.
- **Imports are ignored for now** in the v1beta1 path (component-config imports are
  a large, separate v1beta1 feature).
- **No `--api-version` flag yet.** Because the v1beta1 branch converts down to
  v1alpha1, `dev inspect definition` will *print the v1alpha1 view* of a v1beta1
  package in this milestone. Native v1beta1 output is a deliberate follow-up (see
  Consequences + Roadmap).

## Architecture

```
load.PackageDefinition(ctx, path, opts)            // exported, returns DefinedPackage (v1alpha1)
  ├─ ResolvePackagePath + read manifest bytes      // shared
  ├─ version := pkgcfg.APIVersion(bytes)            // pkgcfg reports the type
  └─ switch version
       ├─ "" | zarf.dev/v1alpha1 → v1alpha1PackageDefinition(...)   // current body, unchanged
       └─ zarf.dev/v1beta1       → v1beta1PackageDefinition(...)    // new
                                     parse(v1beta1) → validate(v1beta1)
                                     → convert.PackageV1Beta1ToV1Alpha1
                                     → DefinedPackage{Pkg: v1alpha1}
```

## Implementation steps

### 1. `pkgcfg`: detection + v1beta1 decode (`src/internal/pkgcfg/pkgcfg.go`)

- Add exported `APIVersion(b []byte) (string, error)` — parses the single doc via the
  existing `parseZarfYAMLDocs` + `apiVersionFromNode` and returns the raw apiVersion
  (empty string means unset → caller treats as v1alpha1). This is the "return the type"
  step the caller branches on.
- Add `ParseV1Beta1(ctx context.Context, b []byte) (v1beta1.Package, error)` — decodes a
  single v1beta1 document with `goyaml.NodeToValue` into `v1beta1.Package`. No v1alpha1
  migrations (those are v1alpha1-only).
- Leave `Parse`, `ParseMultiDoc`, and `knownAPIVersions` unchanged. The existing v1alpha1
  pipeline keeps rejecting v1beta1 cleanly until later milestones promote it.

### 2. `load`: dispatcher + per-version functions (`src/pkg/packager/load/load.go`)

- Refactor `PackageDefinition` into a thin dispatcher:
  - `ResolvePackagePath` → read `ManifestFile` bytes (done once, shared).
  - `version := pkgcfg.APIVersion(b)`.
  - `switch`: v1beta1 → `v1beta1PackageDefinition`; default → `v1alpha1PackageDefinition`.
- `v1alpha1PackageDefinition(ctx, b, pkgPath, opts) (DefinedPackage, error)` — the current
  body verbatim: `pkgcfg.Parse` → architecture default → `resolveImports` → Values feature
  check → `fillActiveTemplate` → `validate` → `DefinedPackage{Pkg, ImportedSchemas}`.
- `v1beta1PackageDefinition(ctx, b, pkgPath, opts) (DefinedPackage, error)` — new:
  1. `pkg, err := pkgcfg.ParseV1Beta1(ctx, b)`
  2. `pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)`
  3. Imports: skipped this milestone. If a component declares `.import`, return a clear
     `"component imports are not yet supported for v1beta1"` error rather than silently
     dropping them.
  4. Templating: skipped (v1beta1 uses Go templating / `zarf dev template`, out of scope).
  5. `validateV1Beta1(ctx, pkg, pkgPath.ManifestFile, ...)` — see step 3.
  6. `v1alpha1Pkg := convert.PackageV1Beta1ToV1Alpha1(pkg)`
  7. `return DefinedPackage{Pkg: v1alpha1Pkg}` (ImportedSchemas empty).

### 3. v1beta1 validation (branched)

- v1alpha1 and v1beta1 validate differently, so the v1beta1 branch gets its own
  `validateV1Beta1`. Minimum viable: validate against the generated
  `zarf-v1beta1-package-schema.json` (mirroring how the v1alpha1 path lints).
- Per ZEP-0048, non-schema validation logic belongs in `src/internal/api/v1beta1`
  (alongside `convert.go`); `validate.go` does not exist there yet. Start minimal
  (schema only) and grow v1beta1-specific rules there over time.
- Validate the v1beta1 struct **before** converting down. Do **not** re-run v1alpha1
  validation on the converted result.

### 4. Command layer

- No changes to `cmd/dev.go`. `devInspectDefinitionOptions.run` already calls
  `load.PackageDefinition`, clears `defined.Pkg.Build`, and prints with
  `utils.ColorPrintYAML`. The dispatch is invisible to it.

## Consequences (call out explicitly)

- In this milestone `dev inspect definition` of a v1beta1 `zarf.yaml` prints the
  **v1alpha1-converted** YAML (because the branch converts down). "Works with a
  v1beta1 package" here means *loads without error and displays*, not *renders as
  v1beta1*. Native v1beta1 output arrives with the `--api-version` follow-up.
- Conversion is lossless for round-trip data (covered by existing convert tests), but
  v1beta1-only constructs render in their v1alpha1 form (e.g. `service` → component
  name conventions, `repositories` objects → repo strings).

## Testing

- **`pkgcfg` unit tests:** `APIVersion` for v1alpha1 / v1beta1 / empty / malformed input;
  `ParseV1Beta1` decodes a known v1beta1 document.
- **`load` unit tests:** `v1beta1PackageDefinition` happy path (parse → validate → convert,
  asserting the returned v1alpha1 package matches expectations); the `.import`-present
  error; confirm the v1alpha1 path is byte-for-byte unchanged.
- **cmd/e2e:** `zarf dev inspect definition` on a testdata dir with a self-contained
  v1beta1 `zarf.yaml`; assert it no longer errors and output parses back. Regression-check
  that a v1alpha1 dir behaves exactly as before.

## Out of scope (this milestone)

- v1beta1 component imports (component-config files).
- v1beta1 package templating (`zarf dev template`, `[[ ]]` delimiters).
- `zarf package inspect definition` for built / OCI / cluster v1beta1 packages
  (needs the multi-doc + `PackageLayout` story).
- `--api-version` flag / native v1beta1 output.

## Roadmap (after Milestone 1)

1. `--api-version` on inspect commands + native-version output (uses `convert` both ways).
2. `zarf package inspect definition` for built/OCI/cluster v1beta1 — aligns with flipping
   the internal currency to v1beta1 and `PackageLayout.Pkg`.
3. v1beta1 `dev lint` + `find-images` on the same local-load foundation.
4. v1beta1 imports (component configs) + `zarf dev template`.
5. Full pipeline migration to v1beta1 as the internal type, then create/deploy + e2e —
   where ZEP-0048's "functions accept the latest version" lands.
