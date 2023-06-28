# NUMBER. TITLE

Date: 2023-05-28

## Status

TBD

## Context

We need a way to decouple transformations from Zarf. We know the Zarf transformation library is battle tested, hardened, and reliable. As Pepr takes admission responsibilities from the Zarf Agent, we need a way to synchronize the TypeScript with the Go code instead of maintaining two disparate libraries which will be expected to grow.

## Decision

We conformed around the idea of using gRPC's protobufs to generate structs (classes) in both languages and use client stubs to call the functions on the gRPC server, which will use the pre-existing transform library.

## Consequences

This inherently decouples the transformation library away from only the code go and allows it to be called from the TypeScript code as well.


Alternatives considered were using [v8go](https://github.com/rogchap/v8go) and executing JavaScript in a sandbox, but this is the inverse of what we need -- we need to execute the go commands on the JavaScript side. There are also c-dependencies associated with v8go.