# 16. Adding a Reports Component

Date: 2023-06-09

## Status

Draft

## Context

When delivering a package in a secure environment oftentimes a security assessor will review things like vulnerabilities, controls, etc. A zarf package does not currently have an easy way to provide documentation, or reports, on things like controls satisfied, vulnerabilities reviewed/justified, SBOM details, etc. While these could be included as static files in a zarf package this would not provide any validation against a schema or ability to deliver these in a non-static-file format (i.e. image attestations).

One specific type of report would be a VEX document. VEX documents/statements provide a standard way for developers to make the status of a vulnerability as it relates to their code. In regulated environments container images are often scanned for vulnerabilities prior to deployment. VEX provides a way to document and justify vulnerabilities found via a scan to help filter down results to only what is truly applicable. Developers can create these VEX statements when building their application for use by a security time during scan time. VEX documents can also be attached to images in a registry as attestations, which provides them directly alongside the artifacts being pulled into an environment. In the optimal scenario - Application vendors and developers provide their applications as container images or other OCI artifacts that could also have VEX documents provided as an attestation.

## Decision

Zarf will include a new `reports` component/noun that will allow for inclusion of specific types of reports. This will provide the ability for zarf to do schema validation against a known schema (ex: openvex spec) during package creation. Additionally the inclusion of these reports in the zarf bundle will allow for end users on the deployment side to review the reports for use in security assessments. A flag (`--type`) has been added to `zarf package inspect` to provide a quick way to view any reports of a given type (ex: vex) for the specified package.

Most reports could also be published to the zarf registry as image attestations or generic OCI artifacts, allowing for further use by other tools (i.e. UI, pipeline automation).

## Consequences

Adding a `reports` noun does separate out the functionality for these types of reports. This could become confusing as https://github.com/defenseunicorns/zarf/issues/475 and similar functionality is evaluated to pull these types of artifacts along with images automatically.

The validation needed by zarf could continue to grow as more report types are supported, which places a burden on the maintenance of zarf to keep these libraries and schemas up to date and track what is supported across different zarf versions.
