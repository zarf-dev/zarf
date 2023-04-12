# Getting Started - VS Code

Zarf uses the [Zarf package schema](https://github.com/defenseunicorns/zarf/blob/main/zarf.schema.json) to define its configuration files. This schema is used to describe package configuration options and enable the validation of configuration files prior to their use in building a Zarf Package.

## Adding Schema Validation

1. Open VS Code.
2. Install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) by RedHat.
3. Open the VS Code command pallete by typing `CTRL/CMD + SHIFT + P`.
4. Type `Preferences: Open User Settings (JSON)`into the search bar to open the `settings.json` file. 
5. Add the below code to the settlings.json config, or modify the existing `yaml.schemas` object to include the Zarf schema.

```json
  "yaml.schemas": {
    "https://raw.githubusercontent.com/defenseunicorns/zarf/main/zarf.schema.json": "zarf.yaml"
  }
```
:::note

When successfully installed, the `yaml.schema` line will match the color of the other lines within the settings.

:::

## Specifying Zarf's Schema Version

To ensure consistent validation of the Zarf schema version in a `zarf.yaml` file, it can be beneficial to lock it to a specific version. This can be achieved by appending the following statement to the **first line** of any given `zarf.yaml` file:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/defenseunicorns/zarf/<VERSION>/zarf.schema.json
```

In the above example, `<VERSION>` should be replaced with the specific [Zarf release](https://github.com/defenseunicorns/zarf/releases).

### Code Example 

![yaml schema](https://user-images.githubusercontent.com/92826525/226490465-1e6a56f7-41c4-45bf-923b-5242fa4ab64e.png)
