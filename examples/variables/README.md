# Variables

This example demonstrates how to define `variables` and `constants` in your package that will be templated across the manifests and charts your package uses during `zarf package deploy` with `###ZARF_VAR_*###` and `###ZARF_CONST_*###`, and also shows how package configuration templates can be used in the `zarf.yaml` during `zarf package create` with `###ZARF_PKG_TMPL_*###`.

With these variables and templating features, you can define values in the `zarf.yaml` file without having to set them manually in every manifest and chart, and can prompt the deploy user for certain information you may want to make dynamic on `zarf package deploy`.

This becomes useful when you are working with an upstream chart that is often changing, or a lot of charts that have slightly different conventions for their values. Now you can standardize all of that from your `zarf.yaml` file.

Text files are also templated during `zarf package deploy` so you can use these variables in any text file that you want to be templated.

:::note

Because files can be deployed without a Kubernetes cluster, some built-in variables such as `###ZARF_REGISTRY###` may not be available if no previous component has required access to the cluster. If you need one of these built-in variables, a prior component will need to have been called that requires access to the cluster, such as `images`, `repos`, `manifests`, `dataInjections`.

:::

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

## Deploy-Time Variables and Constants

To use variables and constants at deploy time you need to have two things:

1. a manifest that you want to template a value in
2. a defined variable in the `zarf.yaml` file from `variables` or `setVariable`

The manifest should have your desired variable name in ALL CAPS prefixed with `###ZARF_VAR`  for `variables` or prefixed with `###ZARF_CONST` for `constants` and suffixed with `###`.  For example in a configmap that took a variable named `DATABASE_USERNAME` you would provide the following:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: db-configmap
data:
  username: ###ZARF_VAR_DATABASE_USERNAME###
```

In the `zarf.yaml`, you would need to define the variable in the `variables` section or as output from an action with `setVariable` with the same `name` as above. Or for a constant you would use the `constants` section.  For the same example as above, I would have the following for a variable defined by the deploy user:

```yaml
variables:
  name: DATABASE_USERNAME
  description: 'The username for the database'
```

And the following for a variable defined as an output from an action:

```yaml
components:
  - name: set-variable-example
    actions:
      onDeploy:
        after:
          - cmd: echo "username-value"
            setVariables:
              - name: DATABASE_USERNAME
```

:::note

All names must match the regex pattern `^[A-Z0-9_]+$` [Test](https://regex101.com/r/BG5ZqW/1)).

:::

:::tip

When not specifying `default`, `prompt`, `sensitive`, or `indent` Zarf will default to `default: ""`, `prompt: false`, `sensitive: false` and `indent: 0`

:::

For user-specified variables, you can also specify a `default` value for the variable to take in case a user does not provide one on deploy, and can specify whether to `prompt` the user for the variable when not using the `--confirm` or `--set` flags.

```yaml
variables:
  name: DATABASE_USERNAME
  default: 'postgres'
  prompt: true
```

:::note

Variables that do not have a default, are not `--set` and are not prompted for during deploy will be replaced with an empty string in manifests/charts/files

:::

For constants, you must specify the value they will use at package create. These values cannot be overridden with `--set` during `zarf package deploy`, but you can use package template variables (described below) to variablize them during `zarf package create`.

```yaml
constants:
  name: DATABASE_TABLE
  value: 'users'
```

:::note

`zarf package create` only templates the `zarf.yaml` file, and `zarf package deploy` only templates other manifests, charts and files

:::

## Create-Time Package Configuration Templates

You can also specify package configuration templates at package create time by including `###_ZARF_PKG_TMPL_*###` as the value for any string-type data in your package definition. These values are discovered during `zarf package create` and will always be prompted for if not using `--confirm` or `--set`. An example of this is below:

```yaml
kind: ZarfPackageConfig
metadata:
  name: 'pkg-variables'
  description: 'Prompt for a variables during package create'

constants:
  - name: PROMPT_IMAGE
    value: '###ZARF_PKG_TMPL_PROMPT_ON_CREATE###'

components:
  - name: zarf-prompt-image
    required: true
    images:
      - '###ZARF_PKG_TMPL_PROMPT_ON_CREATE###'
```

:::note

You can only template string values in this way as non-string values will not marshal/unmarshal properly through the yaml.

:::

:::note

If you use `--confirm` and do not `--set` all of the package configuration templates you will receive an error

:::

:::note

You cannot template the component import path using package configuration templates

:::
