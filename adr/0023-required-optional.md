# 23. Required -> Optional

Date: 2024-01-02

## Status

Pending

## Context

> Feature request: <https://github.com/defenseunicorns/zarf/issues/2059>

Currently, all Zarf components default to being optional due to the `required` key being _optional_ in the yaml. This leads to package authors needing to ensure that they annotate this key for each component, and since nothing in the current validations prompts them about this they may be confused about the "all things are optional" default state.

When Zarf was first created, we didn't really know how it would evolve and this key was introduced in those very early days. At this point it would be better to require all components by default--especially with the introduction of composability and the OCI skeleton work, there is plenty of flexibility in the API to compose bespoke packages assembled from other packages.

A few ways to handle this:

1. Simply force the `required` key to be a non-optional, so that package authors would be forced to specify it for each component, thereby removing any ambiguity--but also force one more key for every single component ever created 🫠

2. Deprecate `required` and introduce an optional `optional` key, which would default to _false_. I do think this still feels strange if you did something like `optional: false`, (to be fair `required: false` has the same awkwardness).

3. Do something more significant like combine various condition-based things such as `only`, `optional` (instead of `required`), or `default`.

## Decision

> The change that we're proposing or have agreed to implement.

## Consequences

> What becomes easier or more difficult to do and any risks introduced by the change that will need to be mitigated.
