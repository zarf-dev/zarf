# Contributing to Zarf

First off, thanks so much for wanting to help out! :tada:

The following is a set of guidelines for contributing to Zarf. These are mostly guidelines, not rules. Use your best judgement, and feel free to propose changes to this document in a pull request.

## Developer Experience

Continuous Delivery is core to our development philosophy. Check out [https://minimumcd.org](https://minimumcd.org/) for a good baseline agreement on what that means.

Specifically:

- We do trunk-based development with short-lived feature branches that originate from the trunk, get merged to the trunk, and are deleted after the merge
- We don't merge code into the trunk that isn't releasable
- We perform automated testing on all changes before they get merged to the trunk
- We create immutable release artifacts

### Developer Workflow

Here's what a typical "day in the life" of a Zarf developer might look like. Keep in mind that other than things that are required through automation these aren't hard-and-fast rules. When in doubt, the process outlined here works for us.

:key: == Required by automation

1. Pick an outstanding issue to work on, set yourself as the assignee, and move it to "Doing Now" in the [Kanban Board](https://github.com/orgs/defenseunicorns/projects/1). The "Ready to Start" and "Planned" columns are mostly prioritized according to feedback from our users and other inputs, but don't feel like you have to pick from the top of the pile if something else is jumping out at you.
1. Write up a rough outline of what you plan to do in the issue so you can get early feedback. Clearly announce any breaking changes you think need to be made.
1. :key: Set up your Git config to GPG sign all commits. [Here's some documentation on how to set it up](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits). You won't be able to merge your PR if you have any unverified commits.
1. :key: Create a branch off the trunk, or a fork if you are an external contributor.
1. Create a Draft Pull Request as soon as you are able to, even if it is just 5 minutes after you started working on it. Around here we aren't afraid to show unfinished work. It helps us get feedback more quickly.
1. :key: Create a Pull Request (or mark it Ready for Review if you've been working in a Draft PR).
1. :key: Run all automated tests by adding `/test all` as a comment in the PR. If you want to re-run certain tests you can also run them individually. Example: `/test e2e` just runs the end-to-end tests. You can chain different tests together like `/test foo bar baz`. You need `write` permission to the repo to trigger the tests.
1. Clearly announce any breaking changes that have been made.
1. :key: Get at least 1 peer-review approval.
1. :key: Merge the PR into the trunk. We tend to prefer "Squash and Merge" but if your commits are on-point and you want to preserve them in the Git history of the trunk that's fine too.
1. Delete the branch
1. Close the issue if it got fully resolved by your PR. *Hint: You can add "Fixes #XX" to the PR description to automatically close an issue when the PR is merged.*

## Testing

This section dives deeper into how we test Zarf

### Pre-Commit Hooks and Linting

In this repo we use [pre-commit](https://pre-commit.com/) hooks for automated validation and linting. The CI pipeline will (eventually) validate that all of the hooks pass so we strongly recommend that you install the hooks locally or you'll be spending a lot of time manually fixing issues that could be fixed automatically very quickly.

#### Pre-Commit Prerequisites

1. Install [pre-commit](https://pre-commit.com/)
1. Install [go](https://golang.org/)
1. Install [golangci-lint](https://github.com/golangci/golangci-lint)
1. Run `pre-commit install` in the repo to install the pre-commit hooks. This will make the hooks run automatically each time you `git commit`. If you want to skip the hooks for any reason you can run `git commit --no-verify` to skip them.

> **HINT:** *Consider [automatically enabling the hooks in every Git repository](https://pre-commit.com/#automatically-enabling-pre-commit-on-repositories)*

### End2End Testing

Our E2E tests utilize [Terratest](https://terratest.gruntwork.io/). They create real infrastructure in AWS that the tests get run on. By doing this we are able to make the test environments ephemeral and allow them to be run in parallel so that we can do more testing more quickly.

The Terratest E2E tests can be found in the `/test` folder.

We're still working on building out the test suite. If you want to help check out these issues:

- [#97](https://github.com/defenseunicorns/zarf/issues/97): Make our own test harness container
- [#99](https://github.com/defenseunicorns/zarf/issues/99): Writing additional tests
- [#100](https://github.com/defenseunicorns/zarf/issues/100): Each test should be runnable locally
- [#101](https://github.com/defenseunicorns/zarf/issues/101): Run E2E tests on multiple distros
- [#102](https://github.com/defenseunicorns/zarf/issues/102): Make the E2E tests more efficient

## Documentation

In this section you'll find documentation on documentation! Pun absolutely intended :smile:

### Architecture Decision Records (ADR)

We've chosen to use ADRs to document architecturally significant decisions. We primarily use the guidance found in [this article by Michael Nygard](http://thinkrelevance.com/blog/2011/11/15/documenting-architecture-decisions) with a couple of tweaks:

- The criteria for when an ADR is needed is undefined. The team will decide when the team needs an ADR.
- We will use the tool [adr-tools](https://github.com/npryce/adr-tools) to make it easier on ourselves to create and maintain ADRs.
- We will keep ADRs in the repository under `docs/adr/NNNN-name-of-adr.md`. `adr-tools` is configured with a dotfile to automatically use this directory and format.

### How to use `adr-tools`

```bash
# Create a new ADR titled "Use Bisquick for all waffle making"
adr new Use Bisquick for all waffle making

# Create a new ADR that supercedes a previous one. Let's say for example that the previous ADR about Bisquick was ADR number 9.
adr new -s 9 Use scratch ingredients for all waffle making

# Create a new ADR that amends a previous one. Let's say the previous one was ADR number 15
adr new -l "15:Amends:Amended by" Use store-bought butter for all waffle making

# Get full help docs. There are all sorts of other helpful commands that help manage the decision log.
adr help
```
