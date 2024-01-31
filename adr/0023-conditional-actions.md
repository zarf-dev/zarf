# 22. Introduce Conditional Actions

Date: 2024-01-31

## Status

Pending

## Context

There are many cases where actions need to be conditional in order to accomplish some objective.  The normal pattern thus far has been to use shell conditionals to achieve this though that results in actions that are no longer cross-platform since POSIX and Windows `if` statements do not line up cleanly.  Below are a few scenarios that this functionality has been requested for:

- Optionally running a command to check or create something (i.e. a `storageclass` if the deployment optionally wants to setup a `pvc`)
- Setting a Zarf variable to a certain value based on a conditional check that will be used in further templating (i.e. looking up values from a command if `USE_X` is set to true)

---

There are a few ways that we can go down for something that answers these needs:

#### 1. Expand Windows Powershell Compatibility to change POSIX `if`s to Windows ones

**Pros**

- This functionality is already in Zarf and would only need to be expanded
- People could use shell syntax that they may already be familiar with

**Cons**

- This is very complicated to actually do in practice
- This doesn't reduce verbosity of conditional checks
- For those not familiar with shell `if`s they can be obtuse

#### 2. Implement similar syntax to a GitHub action `if`

**Pros**

- Relatively familiar and terse syntax
- Reduces verbosity of conditionals from the current state of shell `if`s
- Relatively easy to implement if we used go templating as the expression engine

**Cons**

- We would need to determine how we were going to evaluate the expression (likely go templates instead of js)
- Not as flexible since you can only evaluate a single expression

#### 3. Implement similar syntax to a GitLab job `rule`

**Pros**

- Relatively familiar with the simpler rulesets
- Slightly reduces verbosity of conditionals from the current state of shell `if`s
- Relatively easy to implement if we used go templating as the expression engine

**Cons**

- We would need to determine how we were going to evaluate the expression (likely go templates instead of ruby)
- Adds complexity with how rules can interact that may lead to confusion
- We would need to address the complexity around `when` (or omit it) since we already have onFailure / onSuccess action sets.

#### 4. Implement similar syntax to the `test` command

**Pros**

- Relatively familiar and terse syntax
- We could embed a `zarf tools test` subcommand
- Reduces verbosity of conditionals from the current state of shell `if`s

**Cons**

- Not familiar to everyone and can be obtuse like shell `if`s at times
- No `test` expression libraries exist in go so we would need to write our own
- Not as flexible since you can only evaluate a single expression

## Decision



## Consequences
