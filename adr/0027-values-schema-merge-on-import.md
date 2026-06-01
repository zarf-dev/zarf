# 27. Values Schema Merge on Component Import

Date: 2026-06-01

## Status

Accepted

## Context

Zarf supports package-level `values.yaml` files that supply Helm override values, and a companion
`values.schema.json` that validates them. When a parent package imports a component from a child
package, Zarf already merges the child's values files into the parent's (child first, parent wins).
However, the child's `values.schema.json` was previously silently ignored — only the parent's schema
was copied into the assembled package.

This asymmetry creates a structural gap for skeleton packages. A skeleton is a reusable component
library published via OCI. Its entire purpose is to be imported by a parent and to contribute
configuration requirements. Without schema propagation, a skeleton cannot declare which values it
requires; the parent author must duplicate those constraints in their own schema or leave them
undeclared entirely.

The relevant issue is [zarf-dev/zarf#4877](https://github.com/zarf-dev/zarf/issues/4877).

### Requirements

- When a parent package imports a child component and both define `values-schema.json`, the assembled
  package must contain a single merged schema that satisfies both contracts.
- The parent's schema wins on conflicts (matching the existing values-file precedence model).
- Child `required` constraints must survive the merge — a child declaring `required: ["registry"]`
  must cause the merged schema to enforce that field even if the parent does not mention it.
- Deep import chains (A imports B imports C, each with a schema) must propagate transitively.
- The initial scope is **path-based imports only**. OCI skeleton URL imports with schemas are
  explicitly rejected until the OCI fetch path can supply the schema bytes at import time.

### Constraints

- `values.schema.json` is a user-authored JSON Schema file (Draft-07 in practice). Users may use
  `$ref` pointers to split a large schema across multiple files.
- `$ref` resolution in the context of schema merging introduces two problems: (1) the merge
  algorithm operates on raw `map[string]any` and cannot recurse into unresolved references, and
  (2) relative `$ref` paths inside a child schema point to files in the child's source directory,
  which is not present in the assembled package layout. Resolving these at assembly time would
  require a full JSON Schema dereferencer and transitive file copying, which is out of scope for
  the initial implementation.

---

## Decision

### 1. Transient Schema Carrier on `ZarfValues`

An exported field `ImportedSchemas []string` is added to `ZarfValues` in
`src/api/v1alpha1/package.go` with `json:"-" yaml:"-"` tags. It is excluded from all
marshaling and therefore does not appear in `zarf.yaml` on disk or in the published JSON Schema.
Its sole purpose is to carry child schema paths from import resolution through to package assembly
within a single `zarf package create` execution.

This mirrors the pattern used by `Values.Files` (the values file slice) but avoids changing the
on-disk format. The alternative of passing schemas as a separate parameter to `AssemblePackage`
would have required signature changes across multiple callers.

### 2. Schema Path Collection in `resolveImports`

`resolveImports` (`src/pkg/packager/load/import.go`) is extended to collect child schema paths
alongside the existing values file collection:

```
if importedPkg.Values.Schema != "" {
    importedSchemas = append(importedSchemas, makePathRelativeTo(importedPkg.Values.Schema, importPath))
}
for _, s := range importedPkg.Values.ImportedSchemas {
    importedSchemas = append(importedSchemas, makePathRelativeTo(s, importPath))
}
```

Since `resolveImports` is recursive, `importedPkg.Values.ImportedSchemas` already contains
transitively resolved grandchild schemas from deeper import levels. Path-fixing via
`makePathRelativeTo` is applied at each level, producing paths relative to the top-level package
root by the time the function returns. The final slice is assigned to `pkg.Values.ImportedSchemas`
after the component loop.

**Import order:** The child's own schema is appended before the schemas it collected from its own
imports. This means among child schemas, the directly-imported child takes precedence over deeper
transitive imports (earlier entry in the slice wins during merge).

### 3. Merge Semantics

`MergeSchemas(parent, child map[string]any) map[string]any` is implemented in
`src/pkg/value/schema.go`. Rules:

| JSON Schema keyword | Merge behavior |
|---|---|
| `properties` | Recursively merged; parent wins on the same property key at every depth |
| `required` | Union of both arrays, deduplicated; order: parent items first, then child-only items |
| All other keys (`type`, `description`, `additionalProperties`, `allOf`, etc.) | Parent wins; child value is used only when the key is absent from parent |

The `required` union rule is the key design choice. `required` is a constraint array, not a
property definition, so "parent wins on conflicts" does not apply the same way — there is no JSON
Schema concept of "explicitly not required". If a child declares `required: ["registry"]` and the
parent does not, the merged schema must enforce `registry`, or the feature provides no value to
skeleton authors.

### 4. `$ref` Restriction for Imported Schemas

`CheckNoRefs(schema map[string]any) error` in `src/pkg/value/schema.go` walks the schema object
and returns an error if any `$ref` key is encountered at any depth.

This check is applied to **all schemas** — both imported child schemas and the parent schema —
regardless of whether merging is required. A parent schema with `$ref` is rejected at create time
even when no child schemas are present, because the assembled package may be deployed without the
referenced files available.

The error message directs users to flatten their schema before importing.

### 5. Assembly: `mergeAndWriteValuesSchema`

`copyValuesSchema` in `src/pkg/packager/layout/assemble.go` is replaced by
`mergeAndWriteValuesSchema(ctx, parentSchema, importedSchemas, packagePath, buildPath)`.

Behavior matrix:

| Condition | Outcome |
|---|---|
| No parent schema, no imported schemas | No-op |
| Parent schema only (no imports) | Validate + check no `$ref`; copy verbatim |
| Imported schemas only (no parent schema) | Validate + check no `$ref` on each child; merge left-to-right; write JSON |
| Parent schema and imported schemas | Validate + check no `$ref` on all; merge children; merge parent on top (parent wins); write JSON |

The assembled `values.schema.json` is written as pretty-printed JSON (`json.MarshalIndent`).
The `values.schema` field in the assembled `zarf.yaml` is left as-is (pointing to the original
parent schema path, or empty if the parent had none) — it is metadata about the source file, not
the assembled artifact path.

---

## Consequences

### What becomes easier

- **Skeleton package contracts**: Skeleton authors can declare `required` fields in their schema.
  Importers receive a merged schema that enforces both the skeleton's constraints and the parent's
  without manual duplication.

- **Deep import chains**: Schema constraints accumulate transitively. A three-level chain
  (top → middle → bottom) produces a single merged schema in the assembled package containing
  constraints from all levels, with the top-level package winning on all conflicts.

- **No on-disk format change**: `ImportedSchemas` is a runtime-only field. Existing `zarf.yaml`
  files, published packages, and tooling that reads the schema are unaffected.

### Constraints introduced

- **`$ref` not supported in merged schemas**: Child schemas (and the parent schema when merging)
  must be self-contained, single-file JSON Schema documents. Authors who currently use `$ref` to
  split a large schema must flatten it before the package is importable. A clear error at create
  time guides this.

- **OCI skeleton schemas deferred**: The restriction at `import.go:139` (error on skeleton imports
  with `Values.Schema`) is unchanged. OCI skeletons with schema files will continue to fail until
  the OCI fetch path exposes the schema bytes at import resolution time (the schema is embedded in
  the OCI layer, not the `zarf.yaml`, and is not available during `resolveImports`).

- **Child-vs-child ordering is first-wins among siblings**: When a parent imports two sibling
  components that each declare a schema, the first component in the `components` list wins on
  conflicts between those two schemas. This is deterministic and follows the existing left-to-right
  ordering used for values files, but may be surprising if two sibling components define
  overlapping property constraints.

- **Assembled schema is not the source file**: The merged `values.schema.json` in the package
  layout is a synthetic artifact, not a verbatim copy of any authored file. Tools that read
  `pkg.Values.Schema` from the assembled `zarf.yaml` and expect it to match the file on disk will
  find the path points to the original source location (which may not exist in a deployed context).
  Consumers should read `layout.ValuesSchema` from the package layout directly.

### Testing

Unit tests cover `CheckNoRefs` and `MergeSchemas` directly in `src/pkg/value/schema_test.go`.

Import resolution tests are in `src/pkg/packager/load/import_test.go`:
- `TestResolveImportsSchemaCollection` covers the three main cases: child-only schema, parent+child
  schema, and a 3-level deep import chain with transitive propagation.
- `TestResolveImports` (existing) is updated to clear the transient `ImportedSchemas` field before
  structural comparison, since `expected.yaml` fixtures do not and cannot encode runtime-only state.
