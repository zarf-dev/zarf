# Developing Zarf Packages

## `dev` Commands

The `dev` commands are meant to be used in **development** environments / workflows. They are **not** meant to be used in **production** environments / workflows.

### `dev deploy`

The `dev deploy` command will combine the lifecycle of `package create` and `package deploy` into a single command. This command will:

- Not result in a re-usable tarball / OCI artifact
- Not have any interactive prompts
- Not require `zarf init` to be run (by default, but _is required_ if `--yolo` is not set)
- Be able to create+deploy a package in either YOLO mode (default) or prod mode (exposed via `--yolo` flag)
- Only build + deploy components that _will_ be deployed (contrasting with `package create` which builds _all_ components regardless of whether they will be deployed)

```bash
# Create and deploy dos-games in yolo mode
$ zarf dev deploy examples/dos-games
```

```bash
# If deploying a package in prod mode, `zarf init` must be run first
$ zarf init --confirm
# create and deploy dos-games in prod mode
$ zarf dev deploy examples/dos-games --yolo=false
```

### `dev find-images`

> insert docs here

### Misc `dev` Commands

Not all `dev` commands have been mentioned here.

Further `dev` commands can be discovered by running `zarf dev --help`.
