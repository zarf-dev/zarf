# Zarf Deprecation and Compatibility Policy

**Status:** Draft for Zarf v1.0.0

## Scope

Zarf's explicit SemVer compatibility contract is limited to the package lifecycle:

1. The package definitions accepted by `zarf package create`.
2. The package artifacts produced by supported `zarf package create` workflows.
3. The package artifacts accepted by `zarf package deploy`.
4. The observable deployment and runtime behavior of those artifacts.

A package artifact includes the metadata, layout, signatures, and OCI representations required for the supported create and deploy workflows to interoperate.

The following are outside the explicit SemVer compatibility contract:

- Every command, flag, output format, and behavior below `zarf tools`.
- Commands other than `zarf package create` and `zarf package deploy`.
- Configuration-file keys, environment variables, hidden flags, cache layouts, temporary files, and undocumented behavior.
- Go packages and exported symbols, including `src/api`, `src/api/convert`, `src/pkg/signing`, `src/pkg/oci`, and `src/pkg/zoci`.
- The implementation, command syntax, flags, defaults, and output of standalone signing, verification, publishing, and pulling commands.

Code outside this scope may still follow normal project deprecation practices. It does not receive an independent SemVer support promise from this policy.

## Schema Contract

Schema identifiers and Zarf CLI major versions are independent. A schema suffix such as `beta1` does not determine its Zarf support level.

### `zarf.dev/v1beta1`

`zarf.dev/v1beta1` is the generally available package schema for Zarf 1.x, effective with Zarf v1.0.0.

Within Zarf 1.x, Zarf MUST NOT remove a `zarf.dev/v1beta1` field, kind, enumerated value, or documented core behavior, or change it incompatibly. A future incompatible package schema requires a new schema identity.

A new Zarf 1.x release MAY add backward-compatible schema capabilities. A package using semantics or an artifact format introduced after an older CLI release MUST record the required minimum CLI version in its build metadata. An older CLI MUST fail before deployment with the requirement and its reason, rather than deploy with unknown or changed behavior.

### `zarf.dev/v1alpha1`

`zarf.dev/v1alpha1`, including a package definition that omits `apiVersion`, is a pre-GA legacy schema. It is not part of the stable Zarf 1.x schema contract. It is retained only for the migration period below.

- **November 2026:** Zarf v1.0.0 deprecates v1alpha1 and warns when it is used.
- **May 2027:** Alpha create-time operations, including package creation, publishing, and signing, require `--features=v1alpha1=true`.
- **June 2027:** Alpha deploy-time operations require `--features=v1alpha1=true`.
- **August 2027:** v1alpha1 support ends.

`zarf package remove` remains available for v1alpha1 deployments without that feature flag until v1alpha1 support ends. After August 2027, Zarf does not guarantee creation, deployment, removal, inspection, or other management of v1alpha1 packages or deployments.

The temporary v1alpha1 migration path does not make v1alpha1 a stable Zarf 1.x API. Its scheduled retirement therefore does not require a Zarf v2 release.

## Compatibility Directions

### Older supported package on a newer CLI

A later Zarf 1.x CLI MUST create and deploy packages using supported schema capabilities and supported artifact representations from earlier Zarf 1.x releases. For v1alpha1, this guarantee ends according to the published migration timeline.

### Newer package on an older CLI

Zarf does not guarantee that an older CLI can deploy every newer package. When a package depends on a later CLI's schema capability, artifact representation, or runtime behavior, package creation MUST record the minimum required Zarf version and the reason. Package deployment MUST validate that requirement before changing cluster state.

This is a fail-fast compatibility guarantee, not permission to deploy with unknown behavior. A package consumer can upgrade Zarf or choose not to deploy the package.

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
