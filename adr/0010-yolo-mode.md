# 10. YOLO Mode

Date: 2022-12-14

## Status

Accepted

## Context

Zarf was rooted in the idea of declarative K8s deployments for disconnected environments. Many of the design decisions made in Zarf are based on this idea. However, in certain connected environments, Zarf can still be leveraged as a way to define declarative deployments and upgrades without the constraints of disconnected environments. To that end, providing a declarative way to deploy Zarf packages without the need for a Zarf init package would be useful in such environments.

## Decision

YOLO mode is an optional boolean config set in the `metadata` section of the Zarf package manifest. Setting `metadata.yolo=true` will deploy the Zarf package "as is" without needing the Zarf state to exist or the Zarf Agent mutating webhook. Zarf packages with YOLO mode enabled are not allowed to specify components with container images or Git repos and validation will prevent the package from being created.

## Consequences

YOLO mode provides a way for existing, connected clusters to use Zarf for declarative deployments and upgrades because there is no need to perform any Zarf bootstrapping in order to deploy Zarf-packaged workloads. The addition of the `metadata.yolo` config should not affect existing Zarf users as it is entirely optional. Additionally, requiring the `metadata.yolo` config to be set to `true` and not allowing a runtime flag to override it makes it very clear both in `package create` and `package deploy` the intent and usage of the package.
