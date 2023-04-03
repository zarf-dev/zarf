# Getting started - VS Code

Zarf uses its own [schema](https://github.com/defenseunicorns/zarf/blob/main/zarf.schema.json) to define its configuration files. This schema is used to describe package configuration options and can be used to validate the configuration files before they are used to build a Zarf package.

## Adding schema validation

1. Open VS Code
2. Install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) by RedHat.
3. Open the VS Code command pallete by typing `CTRL/CMD + SHIFT + P`.
4. Type `Preferences: Open User Settings (JSON)`into the search bar to open the `settings.json` file. 
5. Add the below to the settlings.json config, or modify the existing `yaml.schemas` object to include the Zarf schema.

```json
  "yaml.schemas": {
    "https://raw.githubusercontent.com/defenseunicorns/zarf/main/zarf.schema.json": "zarf.yaml"
  }
```
:::tip

yaml.schema line turns the same color as other lines in the setting when installed succesfully!!

:::

## Specifying Zarf's schema version

In some cases, it may be beneficial to lock a `zarf.yaml`'s validation to a specific version of the Zarf schema.

This can be accomplished by adding the below to the **first** line of any given `zarf.yaml`.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/defenseunicorns/zarf/<VERSION>/zarf.schema.json
```

Where `<VERSION>` is one of [Zarf's releases](https://github.com/defenseunicorns/zarf/releases).

### Code Example 

![yaml schema](https://user-images.githubusercontent.com/92826525/226490465-1e6a56f7-41c4-45bf-923b-5242fa4ab64e.png)
