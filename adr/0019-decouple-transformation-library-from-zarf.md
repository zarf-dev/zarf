# 19. Decouple Transformation Library from Zarf

Date: 2023-05-28

## Status

Pending

## Context

We need a way to decouple transformations from Zarf. We know the Zarf transformation library is battle tested, hardened, and reliable. As Pepr takes admission responsibilities from the Zarf Agent, we need a way to synchronize the TypeScript with the Go code instead of maintaining two disparate libraries which will be expected to grow.

We considered:

- WASM
- gRPC
- REST
- Rewrite the code in TypeScript

#### WASM

**PROS**

- Shared codebase between TypeScript and Go
- Fast
- API Contract
- No network overhead
**CONS**
- New technology
- Requires new features in Pepr

#### gRPC

**PROS**

- Shared codebase between TypeScript and Go
- Fast
- API Contract
**CONS**
- Network overhead
- Pepr is considering adding a sidecar and we did not want potentially 3 containers in the Pepr Pod.

#### REST

**PROS**

- Shared codebase between TypeScript and Go
- Proven
- API Contract
**CONS**
- Network overhead
- Pepr is considering adding a sidecar and we did not want potentially 3 containers in the Pepr Pod.

#### Rewrite in TypeScript

**PROS**

- Low hanging fruit
**CONS**
- Two codebases to maintain
- TypeScript is not as battle tested as Go

## Decision

We conformed around using WASM because we can leverage the battle tested transform library from Zarf without incurring network overhead cost from pod to pod or container to container communications.

## Consequences

- Requires an Update to Pepr Core to package the WASM file because it is too large to fit in a ConfigMap
- This will require standardization around the objects that are passed from Pepr to the WASM functions to testability and maintainability.
