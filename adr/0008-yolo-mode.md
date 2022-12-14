# 7. Use Rust Binary for Both Injection Stages

Date: 2022-12-14

## Status

In Review


## Context

In certain connected environments, users would like to use Zarf as a way to define declarative deployments and upgrades while bringing their own container registry and Git server. To that end, these users would like to be able to create and deploy Zarf packages without the mutating webhook, Gitea server and Zarf registry. Additionally, these users would like the capability to upgrade their existing, connected, non-Zarf clusters using Zarf packages.

## Decision

YOLO mode is an optional boolean config set in the `metadata` section of the Zarf package manifest. Setting `metadata.yolo=true` will deploy the Zarf package "as is" without setting the Zarf state or mutating any webhooks. YOLO mode does not require a cluster bootstrapped with a Zarf init package and as such does not require the Gitea server, Zarf registry or Zarf Agent running. Zarf packages with YOLO mode enabled are not allowed to specify components with container images or Git repos.

## Consequences

YOLO mode makes it easy for users of existing, connected clusters to use Zarf for declarative deployments and upgrades because there is no need to perform any Zarf bootstrapping in order to deploy Zarf-packaged workloads. The addition of the `metadata.yolo` config should not affect existing Zarf users as it is entirely optional.
