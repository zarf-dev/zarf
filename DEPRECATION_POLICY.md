# Zarf Deprecation and Compatibility Policy

## Scope

Zarf's explicit SemVer compatibility contract is limited to the package lifecycle:

1. The package definitions accepted by `zarf package create`.
2. The package artifacts produced by supported `zarf package create` workflows.
3. The package artifacts accepted by `zarf package deploy`.
4. The observable deployment and runtime behavior of those artifacts.
5. Zarf-managed in-cluster and deployed-package state for supported package deployments.

A later compatible Zarf 1.x release MUST preserve or migrate state written by an earlier supported Zarf 1.x release so that supported package deployments remain manageable.

A package artifact includes the metadata, layout, signatures, and OCI representations required for the supported create and deploy workflows to interoperate.

The following are outside the explicit SemVer compatibility contract:

- Every command, flag, output format, and behavior below `zarf tools`.
- Configuration-file keys, environment variables, hidden flags, cache layouts, temporary files, and undocumented behavior.
- Logging structure and syntax, including levels, field names, message text, and encoding.
- Go packages and exported symbols, including `src/pkg/signing`, `src/pkg/oci`, and `src/pkg/zoci`.
- The implementation, command syntax, flags, defaults, and output of standalone signing, verification, publishing, and pulling commands.

Code outside this scope remains subject to the project-wide deprecation lifecycle below, but does not receive an independent SemVer support promise from this policy.

## Schema Contract

Schema identifiers and Zarf CLI major versions are independent. A schema suffix such as `beta1` does not determine its Zarf support level.

### `zarf.dev/v1beta1`

`zarf.dev/v1beta1` is the generally available package schema for Zarf 1.x, effective with Zarf v1.0.0.

Within Zarf 1.x, Zarf MUST NOT remove a `zarf.dev/v1beta1` field, kind, enumerated value, or documented core behavior, or change it incompatibly. A future incompatible package schema requires a new schema identity.

A new Zarf 1.x release MAY add backward-compatible schema capabilities. A package using semantics or an artifact format introduced after an older CLI release MUST record the required minimum CLI version in its build metadata. An older CLI MUST fail before deployment with the requirement and its reason, rather than deploy with unknown or changed behavior.

### `zarf.dev/v1alpha1`

`zarf.dev/v1alpha1`, including a package definition that omits `apiVersion`, is a pre-GA legacy schema. It is not part of the stable Zarf 1.x schema contract. It is retained only for its migration period.

The v1alpha1 migration and retirement plan is a one-time v1.0 transition documented with that release. It does not define the project's ongoing deprecation lifecycle.

The temporary v1alpha1 migration path does not make v1alpha1 a stable Zarf 1.x API. Its scheduled retirement therefore does not require a Zarf v2 release.

## Deprecation Lifecycle

The following lifecycle applies to deprecated SDK and CLI logic, including logic outside the explicit SemVer contract above.

- **SDK logic** is a Go API or programmatic integration explicitly made available to consumers.
- **CLI logic** is a user-invoked command, subcommand, flag, or documented CLI behavior, including `zarf tools`.

The project MUST retain deprecated SDK logic through at least three subsequent minor releases and deprecated CLI logic through at least six subsequent minor releases. For example, logic deprecated in v1.2.0 remains available in v1.3.0, v1.4.0, and v1.5.0 as SDK logic, so v1.6.0 is its earliest removal release. CLI logic remains available through v1.8.0, so v1.9.0 is its earliest removal release. Maintainers MAY extend either window at their discretion.

A deprecation MUST identify the affected logic, its replacement or removal rationale, the announcing release, and the earliest removal release. SDK deprecations MUST use Go's `Deprecated:` documentation convention. CLI deprecations MUST issue an actionable warning when used. Release notes MUST describe the migration required before removal.

The security exception below is the only exception to these minimum windows.

## Compatibility Directions

### Older supported package on a newer CLI

A later Zarf 1.x CLI MUST create and deploy packages using supported schema capabilities and supported artifact representations from earlier Zarf 1.x releases. For v1alpha1, this guarantee ends according to the published migration timeline.

### Newer package on an older CLI

Zarf does not guarantee that an older CLI can deploy every newer package. When a package depends on a later CLI's schema capability, artifact representation, or runtime behavior, package creation MUST record the minimum required Zarf version and the reason. Package deployment MUST validate that requirement before changing cluster state.

## Signing and OCI Interoperability

Signing and OCI utilities are implementation details, not public Go APIs or stable standalone command interfaces. The artifact they produce or consume is protected when it is necessary for the supported package lifecycle.

For example, a later compatible Zarf 1.x `zarf package deploy` MUST continue to load and verify a valid signed package produced by a supported earlier Zarf package workflow. It MAY change the implementation, dependencies, command flags, cache layout, or retry behavior used to produce that package.

Likewise, a later compatible `zarf package deploy` MUST continue to consume a compatible package artifact stored in OCI by an earlier supported package workflow. It MAY change standalone publish or pull commands and OCI utility APIs.

## Deprecating Stable Package Behavior

A deprecation of `zarf.dev/v1beta1` behavior MUST:

1. Identify the affected package contract and replacement, if one exists.
2. Describe the migration in release notes and package documentation.
3. Preserve the deprecated behavior throughout Zarf 1.x.
4. Add or update compatibility coverage for create, deploy, and runtime behavior.

The project MAY add an automated migration tool. Such tooling MUST either perform a safe transformation or fail with an actionable description of the manual migration required. It MUST NOT claim lossless conversion where removed features cannot be represented in the target schema.

## Security Exception

A breaking change required to respond to a vulnerability is exempt from the major-version rule and MAY ship in a patch or minor Zarf release. This exception applies whether or not the vulnerability has a CVE.

The change MUST be limited to the required remediation. Release notes MUST identify the affected package contract, the impacted releases, the operational change, and required operator action, except where disclosure would increase risk. Security remediation MUST NOT wait for a deprecation period or a major release.

## Policy Changes

Changing this policy changes Zarf's declared public API. A policy change MUST be reviewed as a project decision and documented in this file's history. It MUST NOT silently narrow an existing Zarf 1.x guarantee.
