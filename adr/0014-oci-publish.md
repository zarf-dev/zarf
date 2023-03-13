# 14. Zarf Packages as OCI Artifacts

Date: 2023-03-10

## Status

Accepted

## Context

Zarf packages are currently only available if built locally or if someone has personally shared a package with you. This is not a scalable way to distribute packages. We wanted to find a way to distribute and publish packages in a way that is easily consumable for the majority of users. When considering the goal of being able to share packages, security and trust are very important considerations. We wanted our publishing solution and architecture changes to keep in mind signing of packages and the ability to verify the integrity of packages.

We know we are successful when:
 1. (Priority) Users can use to Zarf can natively publish an OCI compliant Zarf package to an OCI compliant registry
 1. (Secondary goal) Package creators can sign Zarf packages to enable Package deployers can trust a packages supply chain security

## Decision

We decided that changing the structure of Zarf packages to be an OCI artifact would be the best way to distribute and publish packages as we would be able to push, sign, and pull packages from the majority of registries.

Context Driving Change (OCI Packaging):
  - Transform structure of Zarf packages to OCI compliant artifacts
  - Current State: Packages are BLOB (binary large object)
  - OCI compliant packages are native to both kubernetes and package registries.
  - Downstream platforms are converging on using OCI layers as default standard for packaging on the platform.
  - CNCF-defined standard already used.
  - Distribution tools natively support this format already to save custom work and schema for scanning and signing.
  - Helps with overall distribution and allowing zarf packages being uploaded to a docker registry
  - Moving to OCI helps with mission compliance and compatibility for mission partners

## Implementation

A handful of changes were introduced to the structure of Zarf packages.
  1. Each component folder is now a tarball instead of a directory
    - This enables us to treat each component as a layer within the package artifact
  1. Images are now stored in a flattened state instead of an images.tar file
    - This enables us to treat each individual image layer as a layer within the package artifact
  1. SBOM files are now stored in a tarball instead of a directory
    - This enables us to treat the SBOM artifacts as a single layer within the package artifact

## Consequences

Backwards compatibility was an important considering when making these changes. We had to implement logic to make sure a new version of the Zarf binary could still operate with older versions of Zarf packages.
At the moment we are testing the backwards compatibility by virtue of maintaining the `./src/test/e2e/27_cosign_deploy_test.go` where we are deploying an old Zarf package via `sget`.


One thing we may want to look at more in the future is how we can get more intricate tests around the backwards compatibility.
The reason why testing backwards compatibility is difficult is because this isn't a `zarf.yaml` schema change (like we had recently with the 'Scripts to Actions' PR) but an compiled package architecture change. This means that we will either need to maintain an 'old' Zarf package that will follow future `zarf.yaml` schema changes OR we maintain a modified Zarf binary that creates the old package structure.

...
