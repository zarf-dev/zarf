# Getting started - VS Code

Zarf uses its own [schema](https://github.com/defenseunicorns/zarf/blob/master/zarf.schema.json) to define its configuration files. This schema is used to describe package configuration options and can be used to validate the configuration files before they are used to build a Zarf package.

## Adding schema validation

1. Open VS Code's `settings.json` file with `CTRL/CMD + SHIFT + P` and search for `Preferences: Open User Settings (JSON)`.
2. Add the below to your config, or modify the existing `yaml.schemas` object to include the Zarf schema.

:::note

The [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) by RedHat is a prerequisite for this feature.

:::

```json
  "yaml.schemas": {
    "https://github.com/defenseunicorns/zarf/raw/main/zarf.schema.json": "zarf.yaml"
  }
```

## Specifying Zarf's schema version

In some cases, it may be beneficial to lock a `zarf.yaml`'s validation to a specific version of the Zarf schema.

This can be accomplished by adding the below to the **first** line of any given `zarf.yaml`.

```yaml
# yaml-language-server: $schema=https://github.com/defenseunicorns/zarf/raw/<VERSION>/zarf.schema.json
```

Where `<VERSION>` is one of [Zarf's releases](https://github.com/defenseunicorns/zarf/releases).
