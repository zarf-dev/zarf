# Developing Zarf Packages

## `dev` Commands


Zarf contains many commands that are useful while developing a Zarf package to iterate on configuration, discover resources and more!  Below are explanations of some of these commands with the full list discoverable with `zarf dev --help`.

:::caution

The `dev` commands are meant to be used in **development** environments / workflows. They are **not** meant to be used in **production** environments / workflows.

:::

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
# Create and deploy dos-games in prod mode
$ zarf dev deploy examples/dos-games --yolo=false
```

### `dev find-images`

Evaluates components in a `zarf.yaml` to identify images specified in their helm charts and manifests.

Components that have `git` repositories that host helm charts can be processed by providing the `--repo-chart-path`.

```bash
$ zarf dev find-images examples/wordpress

components:

  - name: wordpress
    images:
      - docker.io/bitnami/apache-exporter:0.13.3-debian-11-r2
      - docker.io/bitnami/mariadb:10.11.2-debian-11-r21
      - docker.io/bitnami/wordpress:6.2.0-debian-11-r18
```

### Misc `dev` Commands

Not all `dev` commands have been mentioned here.

Further `dev` commands can be discovered by running `zarf dev --help`.
