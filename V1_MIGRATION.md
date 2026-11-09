# Zarf v1 Transition and Migration

**Status:** Draft for Zarf v1.0.0

This document is the v1 transition document required by [the deprecation and compatibility policy](DEPRECATION_POLICY.md). It defines the v1 stable package contract and the one-time migration from the legacy schema. It does not change the project-wide deprecation lifecycle.

## Stable v1 Package Contract

`zarf.dev/v1beta1` is the generally available package schema for Zarf 1.x, effective with Zarf v1.0.0. No `zarf.dev/v1` schema is planned solely to rename that identifier.

Within Zarf 1.x, Zarf MUST NOT remove or incompatibly change a `zarf.dev/v1beta1` field, kind, enumerated value, or documented core behavior. A future incompatible package schema requires a new schema identity.

## Legacy v1alpha1 Migration

`zarf.dev/v1alpha1`, including package definitions that omit `apiVersion`, is a pre-GA legacy schema. It is not part of the stable Zarf 1.x package contract.

Zarf v1.0.0 deprecates v1alpha1 and warns when it is used. The v1alpha1 compatibility path is a CLI capability for this transition and follows the six-minor-release CLI deprecation window in `DEPRECATION_POLICY.md`. Zarf MUST support it through v1.6.x; v1.7.0 is the earliest release that MAY end its support. Maintainers MAY extend this window.

During the v1alpha1 support window, Zarf MUST NOT require an opt-in feature flag for v1alpha1 package operations. `zarf package remove` MUST remain available for v1alpha1 deployments through the final supported release. The release that ends v1alpha1 support MUST identify the end of support and required migration in its release notes. After that release, Zarf does not guarantee creation, deployment, removal, inspection, or other management of v1alpha1 packages or deployments.

## Required Migration

Package authors MUST migrate v1alpha1 definitions to `zarf.dev/v1beta1` before the support boundary. `zarf dev upgrade` may automate safe field mappings. Where a v1alpha1 feature cannot be represented in v1beta1, the command MUST fail with actionable migration guidance rather than claim a lossless conversion.

Operators MUST recreate or migrate v1alpha1 packages before the support boundary. An alpha package's successful deployment before Zarf v1.0.0 does not make it part of the stable Zarf 1.x contract.

## Compatibility During the Transition

During the v1alpha1 support window, a later compatible Zarf 1.x CLI MUST continue to create and deploy v1alpha1 packages and preserve or migrate their managed state so supported deployments remain manageable.

Packages that use a newer schema capability, artifact representation, or runtime behavior MUST declare their minimum CLI version in build metadata. An older CLI MUST fail before deployment when it does not meet that requirement.

## SemVer Effect

The scheduled retirement of the v1alpha1 migration path does not require Zarf v2. Zarf v1.0.0 defines `zarf.dev/v1beta1` as the stable Zarf 1.x schema; v1alpha1 is an explicitly time-bounded legacy compatibility path outside that stable contract.
