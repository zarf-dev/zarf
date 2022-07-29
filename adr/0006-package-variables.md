# 6. package variables

Date: 2022-07-28

## Status

Accepted

## Context

Currently, Zarf only allows variables to be specified within components which introduces the following limitations:

 - Variables are scoped to the component making reuse between components impossible
 - Variables cannot be used for elements within the package definition itself
 - Variables can only be set at create time, not by a user at deploy time

This forces a package creator to copy information within their package/component definitions and also requires them to make bespoke packages per environment even if only small changes are needed to the overall spec (such as changing a domain name).

## Decision

The decision was made to move variable definitions to the package level and to split "variables" into three distinct types:

- Variables (specified with the `variables` yaml key) allow for the templating of component files similar to the component variables before them.  The main changes are that they are now specified at the package level (allowing for reuse between components) and have additional prompting and defaulting features to allow a package creator to ask for more information during `zarf package deploy`.
- Constants (specified with the `constants` yaml key) also template component files, but must be specified at package create time.  This allows a package creator to use the same value in multiple places without the need for copying it and without the package deployer being able to override it.
- Package Variables (specified by using `###ZARF_PKG_VAR_*###` in the package definition) allow package creators to template the same information multiple times within the package definition or dynamically specify values or defaults in constants and variables.

## Consequences

This makes it easier to build a single package that will apply to multiple environments and helps package creators to develop automation around their packages while keeping their package definitions DRY.  Choosing to have constants *and* variables also allows us to reduce potential confusion from package deployers who would otherwise be able to accidentally override values that are meant to be static.

As for drawbacks, the largest one is that this provides the potential for a user to build imperative packages depending on what they template or are allowed to template.  This will need to be considered carefully in the future.  The current implementation also ties us to only templating string values for the time being and we will have to think through what should be technically variablizable in the future (for example variables cannot be used in component import paths because this would introduce a lot of fragility for not much perceived user benefit).
