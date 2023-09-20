# 18. Zarf Hooks

Date: 2023-09-20

## Status

Accepted

## Context

The idea of `hooks` is to provide a way for cluster maintainers to register functionality that runs during the deployment lifecycle. Zarf packages already have the concept of `actions` that can execute commands on the host machine's shell during certain package lifecycle events. As `actions` gain more adoption, the team has noticed they are being used to add functionality to Zarf in unexpected ways. We want `actions` to be a tool that extends upon the functionality of Zarf and its packages, not a tool that works around missing or clunky functionality.

We want package creators to be able to create system agnostic packages by leveraging core Zarf functionality. The following is one such scenario:

- _IF_ ECR is chosen as the external registry during `zarf init` / cluster creation, _THEN_ Zarf will seamlessly leverage ECR without requiring advanced user effort.

Using ECR as a remote registry creates 2 problems that Zarf will need to solve:

 1. ECR authentication tokens expire after 12 hours and need to be refreshed. This means the cluster will need to constantly be refreshing its tokens and the user deploying packages will need to make sure they have a valid token.
 2. ECR Image Repositories do not support 'push-to-create'. This means we will need to explicitly create an image repository for every image that is being pushed within the Zarf package.

Packages that get deployed onto a cluster initialized with ECR as its remote registry will need to make sure it solves these 2 problems.

Currently there are 2 solutions:

1. The package deployer solves the problem pre-deployment (creating needed repos, secrets, etc...)
2. The package itself solves these problems with `actions` that are custom built for ECR clusters.

Neither one of these current solutions are ideal. We don't want to require overly complex external + prior actions for Zarf package deployments, and we don't want package creators to have to create and distribute packages that are specific to ECR.

## Decision

[Pepr](https://github.com/defenseunicorns/pepr) will be used to enable custom, or environment-specific, automation tasks to be integrated in the Zarf package deployment lifecycle. Pepr also allows the Zarf codebase to remain agnostic to any third-party APIs or dependencies that may be used.

A `--skip-webhooks` flag has been added to `zarf package deploy` to allow users to opt out of Zarf checking and waiting for any webhooks to complete during package deployments.

## Consequences

While hooks don't introduce raw schema changes to Zarf, it does add complexity where side affects are happening during package deployments that might not be obvious to the package deployer. This is especially the case if the person who deployed the hooks is different from the person who is deploying the subsequent packages.
