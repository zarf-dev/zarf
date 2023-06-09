# 16. Adding a Reports Component

Date: 2023-06-09

## Status

Draft

## Context

When delivering a package in a secure environment oftentimes a security assessor will review details about a package such as the vulnerabilities, security controls, and SBOM. A zarf package does not currently have an easy way to provide reports to aid a security assessor in their review, such as OSCAL (to capture security controls satisfied) or VEX (to capture vulnerabilities reviewed/justified). While these could be included as `files` in a zarf package this would not provide any validation against a schema or ability to deliver these reports in a non-static-file format (i.e. image attestations/OCI).

One specific type of report would be a VEX document. VEX documents/statements provide a standard way for developers to make the status of a vulnerability as it relates to their code. In regulated environments container images are often scanned for vulnerabilities prior to deployment. VEX provides a way to document and justify vulnerabilities found via a scan to help filter down results to only what is truly applicable. Developers can create these VEX statements when building their application for use by a security time during scan time. VEX documents can also be attached to images in a registry as attestations, which provides them directly alongside the artifacts being pulled into an environment. In the optimal scenario - Application vendors and developers provide their applications as container images or other OCI artifacts that could also have VEX documents provided as an attestation.

## Decision

Zarf will include a new `reports` component/noun that will allow for inclusion of arbitrary reports bundled into the zarf package. As applicable the reports could be validated against a known schema (ex: openvex spec, oscal schema) during package creation. Additionally the inclusion of these reports in the zarf bundle will allow for end users on the deployment side to review the reports for use in security assessments. A flag (`--type`) can be added to `zarf package inspect` to provide a quick way to view any reports of a given type (ex: vex) for the specified package.

Most reports will also be published to the zarf registry as generic OCI artifacts (using ORAS), allowing for further use by other tooling such as viewing in a UI, pulling down with the `oras` CLI, etc. In some cases these reports could be attached to images and pushed as attestations.

### Implementation

The reports component in the original zarf.yaml will be used to identify local directories of reports, local report files, and or remote report files.

1. zarf.yaml contains `reports` noun with `source` of directory, file, or remote location
2. `zarf package create` validates file structure and values
3. `zarf package create` injects reports into components directory named using zarf.yaml
4. `zarf package inspect --type [vex, oscal, text] <zarf pkg>` outputs containing report type data
5. `zarf package deploy <zarf pkg>` pushes reports to zarf registry using ORAS (each component as one OCI tag)

## Consequences

### Ambiguity

Adding a `reports` noun separates out the functionality for certain types of reports, but is ambiguous on intended functionality. This could become confusing as https://github.com/defenseunicorns/zarf/issues/475 is evaluated to pull similar types of "reports" as annotations along with images automatically.

**Solution**: The `reports` noun can be used as a way to achieve this functionality today, with the option to review this decision in the future as attestations are pulled automatically. The benefit of maintaining a `reports` noun would be the ability to add other types of reports that aren't normally attached to an image, as well as schema validation based on specific report types.

### Scaling

This solution does not scale beyond Zarf.  Supporting additional report types, or enhancing the flow can only be accomplished with modifications to the Zarf code base.

**Solution**: Mature the Zarf extensions capability to accept non-baked in modules with the ability to pull in extensions from remote sources. By making vulnerability report review, justifications, and dash boarding a standalone product we support many Defense Unicorn projects as well as any outside projects if open-sourced.  Any enhancements to the reports product will not involve modifications to Zarf as long as it adheres to the standard extensions requirements (standards that are not yet developed).
