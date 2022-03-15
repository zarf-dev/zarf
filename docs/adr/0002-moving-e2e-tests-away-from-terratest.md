# 2. Moving E2E Tests Away From Terratest

Date: 2022-03-04

## Status

Accepted

## Context

In previous releases of Zarf, the creation of the initialization package at the core of many of our E2E tests required repository secrets to login to registry1. Since this is an open-source project, anyone could submit a change to one of our GitHub workflows that could steal our secrets. In order to protect our secrets from any bad-actors we used [peter-evans/slash-command-dispatch@v2](https://github.com/peter-evans/slash-command-dispatch) so that only a maintainer would have the ability to run the E2E tests when a PR is submitted for review.

In the current version of Zarf (v0.15) images from registry1 are no longer needed to create the zarf-init-<arch>.tar.zst. This means, given our current span of E2E tests, we no longer need to use repository secrets when running tests. This gives us the ability to reassess the way we do our E2E testing.

When considering how to handle the tests, some of the important additions we were considering were:
  1. Ability to test against different kubernetes distributions
  2. Ability to test against different linux distributions
  3. Ability to run (at least some of) the E2E tests locally without relying on an ec2 instance - for quicker feedback loops when developing new features

## Decision

The previous E2E test code was not extensible enough to be reused to test Zarf against different kubernetes distributions. The test suite was refactored so that we could write a setup and teardown function for each kubernetes distribution we wanted to verify against and the test suite was then responsible for cycling through the different distributions. This gives us the ability to test multiple kubernetes distributions against the same exact test cases.

The individual test cases were also rewritten to not rely on terratest running a bash command over ssh. Instead, the test uses the locally built Zarf binary and example packages to validate expected behavior. This approach works both on local dev machines (linux/mac) and on the Ubuntu GitHub Runner that gets triggered when a pull request is created. This also has the positive side effect of not needing to wait several minutes for an ec2 instance to spin up for testing.

Since we no longer need repository secrets to run the E2E tests, we removed the requirement for a maintainer to use a `/test all` chatops command to dispatch the tests. Instead, there is a new test workflow defined for each kubernetes distribution we are verifying against and the tests get run automatically whenever a PR is created or updated.

When looking back at the list of 'important additions' we were considering above. All three are addressed with this approach. Testing against different kubernetes distributions is as simple as defining how to create and destroy the cluster. All of the test cases are runnable locally and then because of that testing on a linux distribution is possible by just switching over to another machine and running the same `make test-e2e` there. This also gives us the ability to test against cloud distributions like EKS! All you need is a valid kubeconfig and running `go test ./...` in the `./test/e2e` directory will run all of the test cases against the EKS cluster.

## Consequences

While it was not something we were doing before, testing directly on the GitHub Runner instead of using Terratest to test on an ec2 instance means that when we get around to adding automated testing of Zarf against different linux distrobutions we will want to have more discussions on if we want to use self-hosted runners of different OS's or if we want to go back to Terratest to stand up ec2 instances with different AMIs.

In the future, we will likely want to write E2E tests that use images that require repository secrets to access. When that happens we will want to bring back some form of 'maintainer action' to initiate the test workflow. Going back to [peter-evans/slash-command-dispatch@v2](https://github.com/peter-evans/slash-command-dispatch) might be the right answer for that but there will need to be more discussion to make sure the team agrees that's the best solution first.

As the amount of E2E tests we have grows, the longer it will take to get results back on each PR. Duhh! Paralyzing the tests on a single host will be difficult (but not impossible) because it means more logic will be needed in the test suite to make sure the host has enough resources to handle multiple clusters being run in parallel. The simpler solution would be to break out each of our tests into a new GitHub Workflow. This will easily mitigate the issue of tests taking long to run but also give us a lot more yaml to maintain. Either solution is valid but more team discussions will be needed as we get closer to needing it.
