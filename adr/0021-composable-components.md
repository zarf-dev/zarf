# 21. Composable Components

Date: 2023-10-26

## Status

Accepted

## Context

Zarf has supports composing components together between packages on `zarf package create` since v0.16.0.  This has allowed package creators to make more complex packages from smaller reusable bits.  As this functionality grew however there were a few problems that developed:

1. Import chains did not handle scaling to larger numbers of layers with test coverage usually only covering the first import.
2. When OCI skeletons were added they were largely bolted on after the fact without rethinking how they would impact composability.
3. Component filtering via the `only` filter was not implemented in a central location leading to bugs with create-time filters.

## Decision

We decided to separate composability into its own package that represents a composability import chain as a doubly linked list.  This allows us to represent the whole chain as it exists relative to the "head" Zarf package (the definition that Zarf was asked to build) to more easily handle packages that are in different locations (such as OCI skeletons in one's cache).  We also run the compose functions on all components so that the additional filter logic that is needed for these components can be handled more concisely and built upon (as it might for `flavor` https://github.com/zarf-dev/zarf/issues/2101).

## Consequences

Maintaining the full context within a linked list does use more memory and some operations on it are less efficient than they could be if we one-shotted the compose.  This is a decent tradeoff however as most import chains won't be longer than 4 or 5 elements in practice and these structs and operations are relatively small.
