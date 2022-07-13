# Package Variables

This example demonstrates how to define variables in your package that will be templated across the manifests and charts that your package uses.

With the templating feature, you can define values in the zarf.yaml file without having to define them in every manifest and chart.
This becomes useful when you are working with an upstream chart that is often changing, or a lot of the charts you use have slightly different conventions for their values. Now you can standardize all of that from your zarf.yaml file.

&nbsp;

## How to Use Package Variables
The 'placeholder' text in the manifest or chart yaml should have your desired key name in all caps with `###ZARF_VAR` prepended and `###` appened at the end.

For example, if I wanted to create a template for a database username (using the variable `name`: `DATABASE_USERNAME`) I would do something like `###ZARF_VAR_DATABASE_USERNAME###` in the manifest or chart yaml.

In the zarf.yaml you would add the name of the variable in the `variables` section (which must match the regex pattern `[A-Z_]*` [Test](https://regex101.com/?regex=%5BA-Z_%5D%2A)). For that same example as above, I would have:

```yaml
variables:
  name: DATABASE_USERNAME
```

> ⚠️ **NOTE:** *You shouldn't include the `###ZARF_VAR` and `###` and the beginning and end*

You can also specify a `default` value for the variable to take in case a user does not provide one, and whether to `prompt` for the user for the variable when not using the `--confirm` flag.

```yaml
variables:
  name: DATABASE_USERNAME
  default: "postgres"
  prompt: true
```
