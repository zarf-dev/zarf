# 23. Required -> Optional

Date: 2024-01-02

## Status

Pending

## Context

> Feature request: <https://github.com/defenseunicorns/zarf/issues/2059>

Currently, all Zarf components default to being optional due to the `required` key being _optional_ in the yaml. This leads to behaviors where a package author must ensure they are careful to annotate this key for each component--though the validation doesn't require them to so assumes a sort of "all things are optional" default state.

When Zarf was first created, we didn't really know how it would evolve and this key was introduced in those very early days. At this point it would be better to require all components by default--especially with the introduction of composability and the OCI skeleton work, there is plenty of flexibility in the API to compose bespoke packages assembled from other packages.

A few ways to handle this:

1. Simply force the `required` key to be a non-optional, so that package authors would be forced to specify it for each component, thereby removing any ambiguity--but also force one more key for every single component every created 🫠

2. Deprecate `required` and introduce an optional `optional` key, which would default to false. I do think this still feels strange if you did something like `optional: false`.

3. Do something more significant like combine various condition-based things such as `only`, `optional` (instead of `required`), or `default`.

## Decision

> The change that we're proposing or have agreed to implement.

## Consequences

> What becomes easier or more difficult to do and any risks introduced by the change that will need to be mitigated.
