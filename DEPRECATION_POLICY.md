# Zarf Deprecation and Compatibility Policy

## Scope

Zarf's explicit SemVer compatibility contract is limited to the package lifecycle:

1. The package definitions accepted by `zarf package create`.
2. The package artifacts produced by supported `zarf package create` workflows.
3. The package artifacts accepted by `zarf package deploy`.
4. The observable deployment and runtime behavior of those artifacts.
5. Zarf-managed in-cluster and deployed-package state for supported package deployments.

A later compatible release in the same supported major version MUST preserve or migrate state written by an earlier supported release so that supported package deployments remain manageable.

A package artifact includes the metadata, layout, signatures, and OCI representations required for the supported create and deploy workflows to interoperate.

The following are outside the explicit SemVer compatibility contract:

- Every command, flag, output format, and behavior below `zarf tools`.
- Configuration-file keys, environment variables, hidden flags, cache layouts, temporary files, and undocumented behavior.
- Logging structure and syntax, including levels, field names, message text, and encoding.
- Go packages and exported symbols, including `src/pkg/signing`, `src/pkg/oci`, and `src/pkg/zoci`.
- The implementation, command syntax, flags, defaults, and output of standalone signing, verification, publishing, and pulling commands.

Code outside this scope remains subject to the project-wide deprecation lifecycle below, but does not receive an independent SemVer support promise from this policy.

## Major Release Transition Documents

A major release MAY introduce a time-bounded legacy compatibility path. The transition document for that major release MUST identify the stable package schema or schemas, the legacy path's support boundary, the required migration, and whether the legacy path belongs to the new major version's stable contract.

## Schema Contract

Schema identifiers and Zarf CLI major versions are independent. A schema suffix such as `beta1` does not determine its support level.

The applicable major release transition document identifies the generally available package schema or schemas. Within that major version, Zarf MUST NOT remove a stable schema field, kind, enumerated value, or documented core behavior, or change it incompatibly. A future incompatible package schema requires a new schema identity.

A new release MAY add backward-compatible schema capabilities. A package using semantics or an artifact format introduced after an older CLI release MUST record the required minimum CLI version in its build metadata. An older CLI MUST fail before deployment with the requirement and its reason, rather than deploy with unknown or changed behavior.

## Deprecation Lifecycle

The following lifecycle applies to deprecated SDK and CLI logic, including logic outside the explicit SemVer contract above.

- **SDK logic** is a Go API or programmatic integration explicitly made available to consumers.
- **CLI logic** is a user-invoked command, subcommand, flag, or documented CLI behavior, including `zarf tools`.

The project MUST retain deprecated SDK logic through at least three subsequent minor releases and deprecated CLI logic through at least six subsequent minor releases. For an item deprecated in `vM.N.0`, the earliest SDK removal release is `vM.(N+4).0` and the earliest CLI removal release is `vM.(N+7).0`. Maintainers MAY extend either window at their discretion.

A deprecation MUST identify the affected logic, its replacement or removal rationale, the announcing release, and the earliest removal release. SDK deprecations MUST use Go's `Deprecated:` documentation convention. CLI deprecations MUST issue an actionable warning when used. Release notes MUST describe the migration required before removal.

The security exception below is the only exception to these minimum windows.

## Compatibility Directions

### Older supported package on a newer CLI

A later CLI in the same supported major version MUST create and deploy packages using supported schema capabilities and supported artifact representations from earlier releases in that major version.

### Newer package on an older CLI

Zarf does not guarantee that an older CLI can deploy every newer package. When a package depends on a later CLI's schema capability, artifact representation, or runtime behavior, package creation MUST record the minimum required Zarf version and the reason. Package deployment MUST validate that requirement before changing cluster state.

## Signing and OCI Interoperability

Signing and OCI utilities are implementation details, not public Go APIs or stable standalone command interfaces. The artifact they produce or consume is protected when it is necessary for the supported package lifecycle.

A later compatible `zarf package deploy` MUST continue to load and verify a valid signed package produced by an earlier supported Zarf package workflow. It MAY change the implementation, dependencies, command flags, cache layout, or retry behavior used to produce that package.

Likewise, a later compatible `zarf package deploy` MUST continue to consume a compatible package artifact stored in OCI by an earlier supported package workflow. It MAY change standalone publish or pull commands and OCI utility APIs.

## Deprecating Stable Package Behavior

A deprecation of stable package behavior MUST identify the affected package contract and replacement, describe the migration in release notes and package documentation, preserve the deprecated behavior for the remainder of the major version, and add or update compatibility coverage for create, deploy, runtime behavior, and managed state.

The project MAY add an automated migration tool. Such tooling MUST either perform a safe transformation or fail with an actionable description of the manual migration required. It MUST NOT claim lossless conversion where removed features cannot be represented in the target schema.

## Security Exception

A breaking change required to respond to a vulnerability is exempt from the major-version rule and MAY ship in a patch or minor Zarf release. This exception applies whether or not the vulnerability has a CVE.

The change MUST be limited to the required remediation. Release notes MUST identify the affected package contract, the impacted releases, the operational change, and required operator action, except where disclosure would increase risk. Security remediation MUST NOT wait for a deprecation period or a major release.

## Upstream Compatibility Exception

Zarf MAY accelerate a migration or make an otherwise incompatible change within a major release when an upstream dependency removes or incompatibly changes a required capability, including Kubernetes API removals, container-runtime incompatibilities, or OCI specification changes.

Before using this exception, maintainers SHOULD preserve compatibility through an adapter, compatibility layer, or documented workaround where practical. If that is not practical, the release notes MUST identify the upstream change, affected Zarf contract, impacted environments and releases, required operator action, and migration or workaround. The change MUST be limited to restoring compatibility with the supported upstream environment.

This exception does not apply to ordinary dependency upgrades, implementation refactors, or optional upstream capabilities.

## Policy Changes

Changing this policy changes Zarf's declared public API. A policy change MUST be reviewed as a project decision and documented in this file's history. It MUST NOT silently narrow an existing major-version guarantee.
