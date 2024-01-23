# 22. Kubernetes Multi-tenancy with `zarf init`

Date: 2024-01-23

## Status

Pending

## Context

Currently today, when allowing `zarf init` to handle cluster creation, Zarf doesn't have ability to automatically or semi-automatically provision itself across multiple nodes.  The idea here would be to allow for horizontal scalability across multiple virtual or physical nodes for site reliability and automatic failover.

References:

* https://github.com/defenseunicorns/zarf/issues/1041
* https://github.com/defenseunicorns/zarf/issues/1040
* https://github.com/defenseunicorns/zarf/blob/cf9acb50e5a2240e6bd2af994e5904cd0f73fd55/src/pkg/cluster/state.go#L29

## Decision

...

## Consequences

...
