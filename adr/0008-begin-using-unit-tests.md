# 8. Begin using unit tests

Date: 2022-12-12

## Status

Accepted

## Context

There are sections of the code that are not easily able to be tested by our end-to-end testing and now that Zarf is moving into being a library for other products there will be more scrutiny around our defined interfaces.  This ADR explores ways that we could adjust our testing strategy to address these concerns.

Potential considerations:

1. Introduce Unit-Testing Zarf-wide
    Pros:
        - More of the codebase is covered with tests and will force us to take more consideration when updating it
    Cons:
        - As the codebase evolves the efficacy of these tests is likely to go down, especially if they are written to the code rather than the interface

2. Introduce Unit Testing in a defined and limited capacity within Zarf
    Pros:
        - A decent amount of the codebase will be tested (specifically we can target those areas difficult to e2e test)
        - We can be picky about what/where we unit test so that the maintenance burden of the tests is not too high
    Cons:
        - We will need to be vigilant when PRs come through with Unit Tests that these are correct applications of them

3. Introduce Integration Testing or partial End-to-End testing
    Pros:
        - This is closer to how we already test within Zarf
    Cons:
        - These can be relatively complex to orchestrate and maintain over time requiring complex mocks or other mechanisms to make them work

## Decision

It was decided to introduce a limited set of unit tests to the Zarf codebase (2) because this will enable us the most flexibility and control over how we test while also allowing us to specifically target specific units without a ton of overhead.  Specifically we will ask the following before implementing a Unit Test:

1. Is what I want to test a true unit (i.e. a single function or file)?
2. Does what I want to test have a clearly defined interface (i.e. a public specification)?
3. Is this code inside of the `src/pkg` folder or should it be?

## Consequences

This will help us to test things like specific file formats or interactions with certain standards more easily and will allow us to define our own function signatures that are well tested for use in library code.  We will need to be vigilant though going forward that any new test follows the guidelines above, and that contributors are aware of when they should introduce a unit test.
