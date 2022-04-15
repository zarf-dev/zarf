# 4. SBOM Generation with Witness

Date: 2022-03-29

## Status

Accepted

## Context

SBOM are required for software running on government hardware per EO14028.

## Decision

Using Witness' Syft attestor functionality allows Zarf to continue to get more refined SBOM capabilities as Witness' capabilities expand over time. Syft is capable of finding installed packages and some binaries for statically compiled dependencies over each image within a Zarf package. This allows for SBOMs for each image to be generated and packaged along with the Zarf package.  Abilities to export the SBOM to SDPX and CycloneDX formatted documents as well as a browsable web page are in works.

## Consequences

Added depenencies of Witness and Syft which may inflate Zarf binary size.  Increased Zarf package size -- Jeff noted that uncompressed SBOMs for Big Bang Core came in at around 200MB.
