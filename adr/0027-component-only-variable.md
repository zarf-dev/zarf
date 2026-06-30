## 27. Conditional component inclusion via `only.variable`

Date: 2026-06-30

## Status

Proposed

## Context

`only` already gates components on values known at create time (`flavor`) or at the
start of deploy (`localOS`, `cluster.architecture`). There is no way to gate a
component on the value of a package variable or constant resolved at deploy time.

Operators frequently want a single package whose deploy plan is narrowed by an
operator-supplied input: "deploy the component only when `CUSTOM_ATTR=true`,"
"include the migration step only when `MODE=upgrade`," and so on. Today the only
workarounds are:

- Build separate packages per scenario (defeats the point of one airgap artifact).
- Use `--components` to opt in by name (fragile; operators need to know exact
  component names).
- Use `actions` with conditional `cmd` shells (works for actions but not for
  `manifests`, `charts`, `files`, `images`, or `repos`).

A variable-driven `only` filter closes that gap. The complication is ordering:
`only` is evaluated when filtering the component list, but variables are
resolved after filtering today, and the interactive component-selection prompt
runs before variable prompts. So a faithful implementation needs the deploy
pipeline to resolve variables *before* applying the gating filter.

## Decision

Add `only.variable map[string]string` to `ZarfComponentOnlyTarget` in both
`v1alpha1` and `v1beta1`. Compare values as strings (variables are strings
end-to-end, and `--set` already produces `map[string]string`).

Filtering semantics: a component is kept when every `(key, value)` pair in
`only.variable` matches the resolved value at deploy time. Missing keys resolve
to the empty string. Both declared variables and constants are eligible;
constants are useful for static gates without `--set`.

Reshape the deploy pipeline so the order in `packager.Deploy()` (and
`packager.DevDeploy()`) is:

1. Apply `ByLocalOS` (existing).
2. `getPopulatedVariableConfig` — consumes `--set`, then defaults, then prompts.
3. Apply `ByVariable` using the resolved values.
4. Apply `ForDeploy` (optional-component picker, interactive when applicable).
5. Continue deploying.

`ForDeploy` was previously applied in the `cmd` layer (both at LoadPackage time
when `--confirm`, and again after `confirmDeploy` in the interactive branch).
The interactive application is removed; `packager.Deploy` now owns the picker.
`OptionalComponents` is added to `DeployOptions` to plumb the `--components`
flag through. The non-interactive load-time `ForDeploy(false)` filter is kept
as a layer-pull optimization; re-applying `ForDeploy(false)` inside `Deploy()`
is idempotent.

The filter is scoped to deploy-time pipelines (deploy, dev, init). It is
intentionally not wired into create, mirror, pull, or remove — deploy variables
aren't bound in those contexts, and `remove` operates from the recorded
deployed-package secret rather than the package's current `only` definition.

Compose/import merges `only.variable` with union semantics; conflicting values
on the same key are an error consistent with how `only.localOS` already behaves.

Lint enforces the `^[A-Z0-9_]+$` key format as a hard error (matching the
existing `Variable.Name` pattern) and emits a warning when a key isn't declared
as a package variable or constant.

## Consequences

Eases: single-package deploys with operator-driven component selection;
interactive flows where prompting the operator for a flag drives what gets
installed; gating on already-defined constants without forcing `--set`.

Costs:

- Interactive deploy now prompts for variables before the optional-component
  picker. This is a deliberate UX change — operators may answer a couple of
  variable prompts they previously saw only after picking components. Existing
  packages without `only.variable` see no behavioral change beyond order.
- All comparisons are string equality. Operators must quote values
  (`"true"`/`"false"`) in YAML. We did not introduce a richer expression
  language (regex, set membership, boolean logic) to keep the surface area
  small and consistent with how `flavor` and `localOS` work today.
- `only.variable` is deploy-only, like `flavor` is create-only. Create-time
  artifacts (images, charts, files) are still bundled even when a deploy-time
  filter will hide the component; this is consistent with the current behavior
  of `localOS` and is acceptable for the airgap use case.
