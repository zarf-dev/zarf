# v1beta1 release audit

Reviewed on 2026-10-09 at commit `ec01b9310a1712399b7380508d336fbc57dff396`.

**Release recommendation:** resolve findings 1–3 before unhiding v1beta1, and address or explicitly document the remaining compatibility and behavioral gaps. Passing existing tests does not cover these release scenarios.

The review used repository-wide searches followed by tracing schema generation, parsing, conversions, local and remote imports, component publishing, package assembly, image and Git handling, deployment, removal, inspection, signing, deployed state, CLI commands, documentation, and existing tests. This report includes shared behavior when it materially affects beta packages; those findings are identified explicitly. No implementation changes were made.

Evidence labels:

- **Reproduced:** exercised the current checkout through the CLI or public Go operations. Registry reproductions used a temporary localhost registry.
- **Conversion reproduced:** exercised public conversion APIs; the resulting discrepancy is established, but deployment with a historical CLI was not run.
- **Code traced:** supported by the referenced execution paths; cluster-dependent behavior was not exercised.

Priorities: **P1** = release blocker for a core supported flow; **P2** = correctness or compatibility issue; **P3** = inconsistency or release readiness gap.

## 1. P1 — Reusing a remote component corrupts resource paths in both imports

**Evidence: Reproduced.**

Sources: [remote reference cache](src/pkg/packager/load/import_v1beta1.go#L201), [import rebasing](src/pkg/packager/load/import_v1beta1.go#L147), [path mutation](src/pkg/packager/load/import_v1beta1_merge.go#L160).

Import the same remote component URL into two differently named package components. Cached configs are copied by value, but their slices and source pointers remain shared. Rebasing the second import mutates the cache and the first resolved component again. A file source becomes:

```text
.zarf/remote-components/<digest>/.zarf/remote-components/<digest>/resources/0/payload.txt
```

`load.Package` returned the corrupted definitions, and `assemble.AssemblePackage` failed with `unable to access remote resource ... no such file or directory`. This affects files, local charts, chart values files, manifests, image archive paths, and potentially action directories stored in the shared spec.

**Follow-up:** treat cached component definitions as immutable and independently copy all mutable nested resources before rebasing. Cover package creation with the same remote component imported under two names.

## 2. P1 — The alpha compatibility document points at nonexistent chart archives

**Evidence: Conversion reproduced; assembly and loading paths traced.**

Sources: [beta chart conversion](src/internal/api/v1beta1/convert.go#L153), [alpha chart conversion](src/internal/api/v1alpha1/convert.go#L570), [archive naming](src/pkg/packager/layout/layout.go#L111), [multi-document serialization](src/pkg/packager/layout/package.go#L140).

Beta charts have no `LegacyVersion`, so assembly names their archives `chart.tgz` and values files `chart-0`. Conversion to alpha copies a Helm repository version or OCI tag into the alpha chart's `version`. An alpha reader then looks for `chart-1.2.3.tgz` and `chart-1.2.3-0` instead.

Public conversion confirmed `chart.tgz` versus `chart-1.2.3.tgz` for the same beta Helm repository chart. This affects ordinary versioned repository charts and tagged OCI charts, despite the embedded alpha document being intended to make beta packages deployable by older clients. The emitted minimum version for a versionless chart does not resolve the disagreement between metadata and actual filenames.

**Follow-up:** make the alpha projection's layout identifiers agree with the assembled artifacts, while preserving source references separately where needed. Verify an assembled beta package using an alpha-only reader and an actual supported older CLI.

## 3. P1 — Init behavior still depends on component names rather than beta `service`

**Evidence: Code traced; cluster deployment not run.**

Sources: [service declarations](src/api/v1beta1/component_config.go#L42), [internal service selection](src/pkg/packager/deploy.go#L362), [init bootstrap branches](src/pkg/packager/deploy.go#L386), [credential update lookup](src/internal/packager/helm/zarf.go#L223).

Beta exposes `service: registry`, `seed-registry`, `injector`, `agent`, and `git-server`, but init's internal service configuration and special bootstrap behavior still switch on names such as `zarf-seed-registry` and `zarf-agent`.

For example, a component named `bootstrap` with `service: seed-registry` passes validation but does not start injection or receive the seed registry image-push exemption. An agent component with a custom name does not receive the agent-specific image handling. Conversely, conventional names trigger these behaviors even without declaring a service. Credential rotation already looks up components by `Service`, so an otherwise working beta init package using conventional names but omitting `service` can later fail service discovery.

**Follow-up:** consistently use service declarations throughout init and service lookup


## 6. P2 — Explicit `false` cannot override imported `true` settings

**Evidence: Reproduced through `zarf dev inspect definition`.**

Sources: [chart merging](src/pkg/packager/load/import_v1beta1_merge.go#L52), [manifest merging](src/pkg/packager/load/import_v1beta1_merge.go#L104).

Import a chart with `skipWait: true` and `skipSchemaValidation: true`, then explicitly set both to `false` in the importing component. The resolved definition retains both as `true`. The same occurs for manifest `skipWait`, `enableTemplating`, and Kustomize flags.

The wire types use plain booleans and merging only handles truthy overrides, so omitted and explicitly false values are indistinguishable. Consumers cannot restore waiting or schema validation, or disable templating inherited from a component.

**Follow-up:** retain authored field presence during override resolution, and define the false-value merge contract before freezing the schema.

## 7. P2 — Schema validation prevents partial chart overrides before imports are merged

**Evidence: Reproduced through `zarf dev inspect definition`.**

Sources: [validation before resolution](src/pkg/packager/load/load.go#L103), [chart source schema](src/api/v1beta1/component.go#L121), [merge support for inherited sources](src/pkg/packager/load/import_v1beta1_merge.go#L52).

An importing component cannot override just an imported chart's namespace, values, or wait settings:

```yaml
charts:
  - name: app
    namespace: overridden
```

The initial schema check requires a chart source even though the merge implementation can inherit it. The CLI fails with `Must validate one and only one schema (oneOf)` before resolving the import. The importing component must repeat the source definition, coupling it to the imported component's implementation.

**Follow-up:** validate authored overrides with a schema that permits partial entries, then enforce complete chart definitions after resolution. Apply the same rule to nested component configs and template validation.

## 9. P2 — An incompatible remote variant can prevent selecting a compatible one

**Evidence: Reproduced using a localhost registry.**

Sources: [fetch candidates before compatibility filtering](src/pkg/packager/load/import_v1beta1.go#L177), [remote platform resolution](src/pkg/packager/load/import_v1beta1.go#L232), [OCI index selection](src/pkg/oci/fetch.go#L19).

Publish one amd64-only component and one arm64-only component under separate references. List both in a package component's remote import variants and create for amd64.

The amd64 variant is usable, but loading the arm64 reference tries to resolve its index for amd64 before variant filtering. Resolution aborts with `no matching manifest was found in the manifest list` rather than selecting the compatible candidate. Every listed remote reference must currently support the active architecture, which defeats this use of the variant list.

**Follow-up:** inspect candidate variant/platform metadata before requiring a matching leaf, and treat a platform mismatch as an incompatible candidate. Preserve errors for genuinely broken references.

## 10. P2 — Component publishing skips semantic validation of the resolved component

**Evidence: Reproduced using `component.Publish` and subsequent remote import.**

Sources: [publish pipeline](src/pkg/component/publish.go#L62), [validated component loading](src/pkg/packager/load/component.go#L82), [semantic checks](src/internal/api/v1beta1/validate.go#L113).

Publishing calls schema-validating `ComponentConfig` and import resolution, but bypasses the semantic validation used by `ComponentDefinition` and package loading. It also does not run a final schema check after merging.

A component containing a local chart with no namespace published successfully. Importing that artifact into a package then failed with `chart "chart" must include a namespace`. Other missed checks include duplicate chart/manifest names, conflicting command/wait actions, and unsupported checksum algorithms.

**Follow-up:** run the full applicable validation on the resolved component before publishing. An artifact successfully published by this CLI should satisfy the same component contracts when imported.

## 11. P2 — Remote component version requirements are neither enforced nor propagated

**Evidence: Reproduced for enforcement; propagation code traced.**

Sources: [declared publish requirements](src/api/v1beta1/component_config.go#L78), [remote import](src/pkg/packager/load/import_v1beta1.go#L232), [acknowledged TODO](src/pkg/packager/load/load.go#L42), [requirements replacement during assembly](src/pkg/packager/assemble/assemble.go#L890).

A published component declaring `publishData.versionRequirements: [{version: v999.0.0, reason: requires future API}]` imported successfully with the default version checks. The import resolution carries its resources and values but not its version requirements, and assembled package requirements are rebuilt from a small set of package features.

Consumers can build and distribute a package using a component whose declared minimum CLI version they do not meet, without retaining that requirement for deployment. Alpha remote imports already have an enforcement path.

**Follow-up:** define and enforce component operation requirements, honor the explicit skip option, and propagate deploy-relevant requirements into the assembled package.

## 12. P2 — Published directory resources lose executable permissions

**Evidence: Reproduced using a localhost registry and `load.Package`.**

Sources: [resource normalization](src/pkg/component/resource_normalizer.go#L161), [layer descriptor creation](src/pkg/component/publish.go#L279), [materialization mode](src/pkg/packager/load/package.go#L182), [directory/file assembly](src/pkg/packager/assemble/assemble.go#L454).

Publish a component whose `files.source` is a directory containing an executable `run.sh`. The local file was `0755`; the materialized remote file was `0600`. Resource layers carry contents and mount paths but no mode metadata, and every layer is written with user read/write permissions.

This changes behavior when moving a working local component to remote publication. The directory's own executable bit is restored during assembly, but its children's executable bits are not. Setting the outer file entry's `executable` does not recursively repair them.

**Follow-up:** preserve executable permissions for directory members in the artifact contract and materialization path. Verify a deployed directory member can actually execute.

## 13. P2 — Empty directory resources publish successfully but cannot be imported

**Evidence: Reproduced using a localhost registry.**

Sources: [directory walking without directory entries](src/pkg/component/resource_normalizer.go#L187), [required layer validation](src/pkg/packager/load/import_v1beta1.go#L310).

A component with `files.source: empty` referencing an existing empty directory published successfully. Importing it failed with `references local resource "resources/0/empty" absent from artifact layers`.

Normalization writes the directory path into the config but produces no layer for it. Empty directories inside otherwise populated resources are also discarded, changing the contents delivered to disk.

**Follow-up:** represent directories explicitly, or reject unsupported directory resources at publication and document the limitation. Preserve local-to-remote behavior for directory trees.

## 14. P2 — An algorithm-only checksum silently disables verification

**Evidence: Reproduced through package creation.**

Sources: [checksum parsing and presence](src/api/package.go#L291), [algorithm-only validation](src/internal/api/v1beta1/validate.go#L116), [verification condition](src/pkg/packager/assemble/assemble.go#L503).

A file with `checksum: "sha256:"` or `"sha512:"` passes schema and semantic checks. Parsing leaves an empty digest; `IsSet()` returns false, and assembly skips checksum verification. `dev inspect definition` even drops the authored checksum on serialization.

A package containing `checksum: "sha256:"` and arbitrary payload bytes was created successfully. A typo or empty substitution in an explicitly supplied checksum is treated as if the author requested no verification.

**Follow-up:** reject a supplied checksum with an empty or malformed digest. Require the appropriate hexadecimal digest length for each supported algorithm.

## 15. P2 — Source/dev loading selects beta from multi-document YAML but validates alpha

**Evidence: Reproduced through `zarf dev inspect definition`.**

Sources: [version selection](src/internal/pkgcfg/pkgcfg.go#L106), [source validation](src/pkg/packager/load/load.go#L98), [first-document schema decoding](src/pkg/lint/schema.go#L48), [multi-document package output](src/pkg/packager/layout/package.go#L140).

For a definition containing alpha first and beta second, version selection correctly chooses beta. The raw schema validator then uses single-document `yaml.Unmarshal` on the entire input, which validates the first alpha document against the beta schema. The CLI fails with `apiVersion must be ... "zarf.dev/v1beta1"`.

Built package loading handles this format, but source/dev workflows do not, even though package serialization emits exactly this document order. This also complicates inspecting an extracted package definition through dev commands. Other header/AST utilities assume the first document.

**Follow-up:** select and validate the same document everywhere, or explicitly constrain source authoring to one document and provide useful errors for assembled multi-document definitions.

## 16. P2 — Invalid `setValues` entries are accepted until after commands execute

**Evidence: Missing-key failure reproduced through package create/deploy; wait-action behavior code traced.**

Sources: [optional key in beta schema type](src/api/v1beta1/component.go#L360), [action validation](src/internal/api/v1beta1/validate.go#L215), [output assignment](src/pkg/packager/actions/actions.go#L129), [path validation](src/pkg/value/value.go#L37).

This definition creates successfully:

```yaml
cmd: echo ran-command
setValues: [{}]
```

Deploy executes the command and logs `action succeeded`, then fails assigning its output because the key is empty. The returned error is only `command "echo ran-command" failed after 0 retries`, obscuring the invalid path. A real command can have side effects before this avoidable configuration failure.

Missing keys and paths without a leading dot are not checked up front. `setValues` on wait actions is also accepted but never applied because wait actions return before output processing.

**Follow-up:** validate output keys and incompatible action options before package creation and command execution; preserve the underlying output-processing error.

## 17. P2 — Action templates cannot access the documented `.Pkg` object

**Evidence: Reproduced through package create/deploy. Shared templating issue.**

Sources: [action template context](src/pkg/packager/actions/actions.go#L62), [package context implementation](src/pkg/template/template.go#L87), [documented package object](site/src/content/docs/ref/package-values.mdx#L326).

A beta action using `enableTemplating: true` and `cmd: echo {{ .Pkg.Metadata.Name }}` creates successfully but fails during deployment with `map has no entry for key "Pkg"`.

Actions receive values, constants, variables, and state, but no package object. File, manifest, and chart values templates receive the package through a different context-building path. The documentation presents `.Pkg` as an available template object without explaining this action limitation.

**Follow-up:** pass the authored package definition into action templating, or explicitly narrow the documented contract and reject unsupported references before starting deployment.

## 18. P2 — Removal actions do not receive baked package values

**Evidence: Reproduced through package create/remove. Shared values lifecycle issue.**

Sources: [CLI removal inputs](src/cmd/package.go#L1624), [removal values initialization](src/pkg/packager/remove.go#L73), [metadata-only source loading](src/pkg/packager/load.go#L239).

Create a beta package with `values.files: [defaults.yaml]`, where `message: baked-default`, and an onRemove action templating `.Values.message`. Removing the artifact with no explicit values flags fails with `map has no entry for key "message"`.

Removal uses only fresh CLI/API values. It neither loads the package's assembled defaults nor restores deploy-time overrides or action-generated values. Values-based cleanup therefore needs users to reconstruct inputs even when the default was included in the artifact. A failure in a later removal hook can happen after chart uninstallations.

**Follow-up:** define the removal values contract, load available baked defaults, and decide how deploy-specific cleanup inputs should be retained or supplied. Document any requirement to repeat values on remove.

## 19. P2 — A failing beta success hook leaves cluster status recorded as succeeded

**Evidence: Code traced; cluster deployment not run. Shared lifecycle issue, especially relevant to beta readiness waits.**

Sources: [success status persistence before hooks](src/pkg/packager/deploy.go#L338), [success hook failure branch](src/pkg/packager/deploy.go#L347), [converted health checks](src/internal/api/v1beta1/convert.go#L440).

The deployer marks a component succeeded and records it in the cluster before running `onDeploy.onSuccess`. If a hook fails, it runs failure hooks and returns an error without changing the persisted status to failed.

Beta has no `after` list, and alpha health checks are converted into beta onSuccess wait actions. A readiness failure in this normal path can therefore yield a failed CLI deployment while the package secret says the component succeeded. Subsequent inspection and automation receive misleading state.

**Follow-up:** persist success only after all required success hooks complete, and route hook failures through the failure-state recording path. Verify an installed chart followed by a failing readiness hook.

## 20. P3 — The values feature gate differs between beta creation and deployment

**Evidence: Reproduced through package create/deploy.**

Sources: [alpha creation feature check](src/pkg/packager/load/load.go#L140), [beta resolution](src/pkg/packager/load/load.go#L169), [deployment feature check](src/pkg/packager/deploy.go#L207), [inspection feature check](src/pkg/packager/inspect.go#L70).

With `--features=values=false`, a beta package containing values files creates successfully, while deploying that artifact with the same flag fails because values are disabled. Alpha creation rejects the equivalent input immediately. The values feature is enabled by default, so this affects explicit opt-out users.

**Follow-up:** choose one beta feature-gate policy and apply it consistently across creation, component operations, inspection, and deployment.

## Additional release readiness gaps

These are concrete publishing/documentation tasks rather than hidden-command bugs:

- Unhide all three related commands: [component](src/cmd/component.go#L26), [dev upgrade](src/cmd/dev_upgrade.go#L27), and [dev template](src/cmd/dev_template.go#L41), plus the [beta schema page](site/src/content/docs/schema/v1beta1-package.mdx#L5).
- The combined editor schema is generated for alpha and beta **packages**, excluding beta `ZarfComponentConfig`; see [schema generation](src/pkg/schema/generate.go#L38). The standalone component schema exists, but there is no equivalent component reference page under `site/src/content/docs/schema/`. Provide a discoverable schema association/reference for component authors.
- Existing [actions](site/src/content/docs/ref/actions.mdx) and [values](site/src/content/docs/ref/package-values.mdx) documentation primarily demonstrates alpha names such as `template`, `templatedValuesFiles`, and `after`. Beta users need version-specific examples for `enableTemplating`, ordered `valuesFiles`, `onSuccess`, component variants/imports, services, and the removal-values contract.
- [Package path resolution](src/pkg/packager/layout/path.go#L24) prefers `zarf.yaml` over `zarf.gen.yaml`. Running `dev template` in an existing package directory can generate a beta definition that subsequent directory-based create/deploy commands ignore. Explain the precedence and migration steps, or make the workflow explicitly select its generated file.
- [The beta API package comment](src/api/v1beta1/package.go#L4) still says the API is not used within Zarf. Update it with the supported release contract.

## Validation performed and limits

A fresh CLI was built from this checkout with `go build -o /tmp/zarf-v1beta1-audit .`.

The following focused test command groups passed, covering 23 packages:

```sh
go test ./src/api/... ./src/internal/api/... ./src/internal/pkgcfg \
  ./src/pkg/packager/load ./src/pkg/packager/layout ./src/pkg/component \
  ./src/pkg/lint ./src/pkg/state ./src/pkg/template

go test ./src/pkg/packager ./src/pkg/packager/assemble \
  ./src/pkg/packager/actions ./src/pkg/packager/filters \
  ./src/internal/packager/helm ./src/internal/packager/requirements \
  ./src/internal/git ./src/pkg/value ./src/cmd
```

Additional temporary public-API/CLI probes exercised the failures described above. They used temporary directories and localhost registries; no reproduction tests or implementation files were added to the repository. Historical CLI binaries, cluster-dependent E2E journeys, actual init bootstrap, and credential rotation were not executed. Compatibility findings establish concrete projection/artifact discrepancies and still warrant validation against the project's supported older CLI versions. No claim is made that every possible beta behavior or cluster integration was exercised.
