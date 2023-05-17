# 16. Zarf Definition of Generally Available

Date: 2023-05-17

## Status

Accepted

## Context

Eventually Zarf needs to become a "Generally Available" v1.x.x product that people can rely on for mission critical operations.  Today, Zarf can be used in these environments, but it requires someone who is more involved in the Zarf lifecycle than a normal consumer/user to make that successful due to the regular introduction of breaking changes and the lack of testing in certain areas.

## Decision

To make Zarf a Generally Available product we need to focus on overall stability and mechanisms to ensure that stability long-term.  "Stability" in this case is both that of the features we release and of the APIs and schemas that we present to consumers of Zarf.

In order to increase this stability we decided on implementing the following:

- Mechanism / branching strategy to backport patch fixes to older minor releases
- Clear definition of `released`, `beta`, and `alpha` features including a matrix of their support across OSes
- End to End testing that covers the `released` features outlined in that feature matrix - this should also be done:
  - across operating systems (specifically: Windows, macOS, Linux)
  - across major k8s distros (specifically: K3d, K3s, Minikube, Kind, EKS, AKS, OpenShift)
  - across registry providers (specifically: Docker Distribution, ECR, ACR)
  - across git providers (specifically: Gitea, GitLab)
- Unit testing that covers our library code (`src/pkg`) for people using Zarf as a library (code coverage metric TBD)
- Mechanisms and tests to not break compatibility with packages built with older versions of Zarf
- Mechanisms and tests to notify users when they may need to upgrade the Zarf Agent (or Pepr capability)
- Mechanisms to ensure users can easily access documentation specific to the version of Zarf they use
- Mechanisms to ensure a more seamless Zarf install experience (i.e. macOS binary signing, `sudo apk add zarf`, `asdf install zarf X.X.X`)
- Regularly published/maintained example package(s) for tutorials/quick install
- Clear definition / strategy for "what Zarf is" including clear docs on how to use `released` features

> ⚠️ **Note**: Some of these items are already addressed in Zarf but were included for completeness.

## Consequences

Once these are in place we will have many mechanisms to manage Zarf's stability over time but are also signing ourselves up to maintain this promise over the long-term which will increase the burden on the team and reduce our overall velocity - this is good / normal as the project matures but we will need to recognize that we won't have as much flexibility once we reach GA.

This will also affect how Zarf is supported / marketed beyond the core team and we should consider how Zarf "GA" will affect those teams and ensure that they are ready to take on any additional burden as well.
