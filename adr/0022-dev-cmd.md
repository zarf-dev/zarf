# 22. Introduce `dev` command

Date: 2023-12-03

## Status

Accepted

## Context

> Feature request: <https://github.com/defenseunicorns/zarf/issues/2169>

The current package development lifecycle is:

1. Create a `zarf.yaml` file and add components
2. Create a package with `zarf package create <dir>`
3. Debug any create errors and resolve by editing `zarf.yaml` and repeating step 2
4. Run `zarf init` to initialize the cluster
5. Deploy the package with `zarf package deploy <tarball>`
6. Debug any deploy errors and resolve by editing `zarf.yaml` and repeating step 2 or 5

If there are deployment errors, the common pattern is to reset the cluster (ex: `k3d cluster delete && k3d cluster create`) and repeat steps 4-6. Re-initializing the cluster, recreating the package, and redeploying the package is tedious and time consuming; especially when the package is large or the change was small.

`zarf package create` is designed around air-gapped environments where the package is created in one environment and deployed in another. Due to this architecture, a package's dependencies _must_ be retrieved and assembled _each_ and _every_ time.

There already exists the concept of [`YOLO` mode](0010-yolo-mode.md), which can build + deploy a package without the need for `zarf init`, and builds without fetching certain heavy dependencies (like Docker images). However, `YOLO` mode is not exposed via CLI flags, and is meant to develop and deploy packages in fully connected environments.

## Decision

Introduce a `dev deploy` command that will combine the lifecycle of `package create` and `package deploy` into a single command. This command will:

- Not result in a re-usable tarball / OCI artifact
- Not have any interactive prompts
- Not require `zarf init` to be run by default (override-able with a CLI flag)
- Be able to create+deploy a package in either YOLO mode (default) or prod mode (exposed via a CLI flag)
- Only build + deploy components that _will_ be deployed (contrasting with `package create` which builds _all_ components regardless of whether they will be deployed)

## Consequences

The `dev deploy` command will make it easier to develop and deploy packages in connected **development** environments. It will also make it easier to debug deployment errors by allowing the user to iterate on the package without having to re-initialize the cluster, re-build and re-deploy the package each cycle.

There is a purpose to placing this functionality behind a new command, rather than adding it to `package create`. Commands under `dev` are meant to be used in **development** environments, and are **not** meant to be used in **production** environments. There is still the possibility that a user will use `dev deploy` in a production environment, but the command name and documentation will make it clear that this is not the intended use case.
