# Package Variables

This example demonstrates how to define `variables` and `constants` in your package that will be templated across the manifests and charts that your package uses during `zarf package deploy` with `###ZARF_VAR_*###` and `###ZARF_CONST_*###` respectively.  It also shows how package-level variables can be used in the `zarf.yaml` during `zarf package create` with `###ZARF_PKG_VAR_*###`.

With this templating feature, you can define values in the zarf.yaml file without having to define them in every manifest and chart, and can prompt the deployer for certain information that may be dynamic on `zarf package deploy`.

This becomes useful when you are working with an upstream chart that is often changing, or a lot of charts that have slightly different conventions for their values. Now you can standardize all of that from your zarf.yaml file.

&nbsp;

## How to Use Deploy-Time Variables and Constants
The 'placeholder' text in the manifest or chart yaml should have your desired key name in all caps with `###ZARF_VAR` prepended and `###` appened for `variables` or `###ZARF_CONST` prepended and `###` appened for `constants`.

For example, if I wanted to create a template for a database username (using the variable `name`: `DATABASE_USERNAME`) I would do something like `###ZARF_VAR_DATABASE_USERNAME###` in the manifest or chart yaml.

In the zarf.yaml you would add the name of the variable in the `variables` section, or the name of the constant in the `constants` section (which both must match the regex pattern `[A-Z_]*` [Test](https://regex101.com/?regex=%5BA-Z_%5D%2A)). For the same example as above, I would have:

```yaml
variables:
  name: DATABASE_USERNAME
```

> ⚠️ **NOTE:** *You shouldn't include the `###ZARF_VAR` and `###` or `###ZARF_CONST` and `###` at the beginning and end of the `name`*

> ⚠️ **NOTE:** *When not specifying `default` or `prompt` Zarf will default to `default: ""` and `prompt: false`*

For variables, you can also specify a `default` value for the variable to take in case a user does not provide one on deploy, and can specify whether to `prompt` the user for the variable when not using the `--confirm` or `--set` flags.

```yaml
variables:
  name: DATABASE_USERNAME
  default: "postgres"
  prompt: true
```

> ⚠️ **NOTE:** *Variables that do not have a default, are not `--set` and are not prompted for during deploy will be left as their template strings in the manifests/charts*

For constants, you must specify the value they will use at package create.  These values cannot be overridden with `--set` during `zarf package deploy`, but you can use package variables (described below) to variablize them during create.

```yaml
constants:
  name: DATABASE_TABLE
  value: "users"
```

> ⚠️ **NOTE:** *`zarf package create` only templates the zarf.yaml file, and `zarf package deploy` only templates other manifests and charts*

## How to Use Create-Time Package Variables

You can also specify variables at package create time by including `###_ZARF_PKG_VAR_*###` in your package definition's string values.  These values are discovered during `zarf package create` and will be prompted for if not using `--confirm` or `--set`.  An example of this is below:

```yaml
kind: ZarfPackageConfig
metadata:
  name: "pkg-variables"
  description: "Prompt for a variables during package create"

constants:
  - name: PROMPT_IMAGE
    value: "###ZARF_PKG_VAR_PROMPT_ON_CREATE###"

components:
  - name: zarf-prompt-image
    required: true
    images:
      - "###ZARF_PKG_VAR_PROMPT_ON_CREATE###"
```

> ⚠️ **NOTE:** *You can only template string values in this way as non-string values will not marshal/unmarshal properly*

> ⚠️ **NOTE:** *If you use `--confirm` and do not `--set` all of the varaibles you will receive an error*

> ⚠️ **NOTE:** *You cannot template the component import path using package variables*
