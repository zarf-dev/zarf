# 4. SBOM Generation with Witness

Date: 2022-03-29

## Status

Accepted

## Context

SBOM are required for software running on government hardware per EO14028.

## Decision

Using Witness' Syft attestor functionality allows Zarf to continue to get more refined SBOM capabilities as Witness' capabilities expand over time. Syft is capable of finding installed packages and some binaries for statically compiled dependencies over each image within a Zarf package. This allows for SBOMs for each image to be generated and packaged along with the Zarf package.  Capabilities to display SBOMs for a Zarf package are forthcoming.

## Consequences

Syft has some dependencies that cause some issues currently.  Namely bumping to k8s-api v0.23 causes some incompatibilities with derailed/popeye which has resulted in a fork being made.  This bump may also be undesirable due to compatibility with clusters that do not use this version of the k8s api.  TestifySec is currently investigating removing the Syft dependency entirely and generating the SBOM without Syft which will alleviate these concerns and conflicts.
