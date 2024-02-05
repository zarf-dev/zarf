# 23. Required -> Optional

Date: 2024-01-02

## Status

Accepted

## Context

> Feature request: <https://github.com/defenseunicorns/zarf/issues/2059>

Currently, all Zarf components default to being optional due to the `required` key being _optional_ in the yaml. This leads to package authors needing to ensure that they annotate this key for each component, and since nothing in the current validations prompts them about this they may be confused about the "all things are optional" default state.

When Zarf was first created, we didn't really know how it would evolve and this key was introduced in those very early days. At this point it would be better to require all components by default--especially with the introduction of composability and the OCI skeleton work, there is plenty of flexibility in the API to compose bespoke packages assembled from other packages.

A few ways to handle this:

1. Simply force the `required` key to be a non-optional, so that package authors would be forced to specify it for each component, thereby removing any ambiguity--but also force one more key for every single component ever created 🫠

2. Deprecate `required` and introduce an optional `optional` key, which would default to _false_.

3. Do something more significant like combine various condition-based things such as `only`, `optional` (instead of `required`), or `default`.

## Decision

Option 2: deprecate `required` and introduce an optional `optional` key, which defaults to _false_.

Components are now **required** by default, instead of **optional**.

## Consequences

`zarf package create` will fail if any usage of `required` is detected in the `zarf.yaml`, resulting in some thrash for package creators.

Packages created w/ Zarf v0.33.0+ will have their implicit _required_ logic flipped from previous versions (implicit `required: false` --> implicit `optional: false`).

A `required-to-optional` migration (both accomplished behind the scenes on a `zarf package create`, or available via **new** CLI migration: `zarf dev migrate <dir> --run required-to-optional`).
