# 16. adding vex component

Date: 2023-04-28

## Status

Pending Review

## Context

VEX attestations are a type of code signing technology that allow developers to verify the authenticity and integrity of code, which can be particularly important in security-sensitive contexts. As such, the addition of VEX attestations to a software repository should be done in a way that maximizes both security and ease-of-use for developers.  VEX technology can be integrated into the build process of software applications, allowing developers to automatically generate and sign VEX attestations when building their code. This can help to streamline the process of adding VEX to software and verifying vulnerabilities, while also ensuring that all code is properly signed and verified before being deployed.

In the optimal scenario - Application vendors and developers would be providing their applications as container images or other OCI artifacts that could also have VEX provided as an attestation.

## Decision

Evaluate Zarf #475 for the ability to write logic that will be transferrable to generic attestation collection.  With focus on the ability to reference VEX documents in the zarf package manifest for a component.

## Consequences

Implementing VEX technology can add complexity to the development process, particularly if developers are not familiar with the technology or are using it for the first time. This can lead to additional time and resources being required to implement VEX properly.

Verifying code using VEX can add additional processing overhead and potentially impact the performance of the software, particularly in cases where a large number of VEX attestations need to be verified at runtime.