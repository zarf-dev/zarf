# Component Variables

This example demonstrates how to define variables in your component that will be templated across the manifests and charts that your component uses.

With the templating feature, you can define values in the zarf.yaml file without having to define them in every manifest and chart.
This becomes really useful when you are working with an upstream chart that is often changing, or a lot of the charts you use have slightly different conventions for their values. Now you can standardize all that from your zarf.yaml file.

&nbsp;

## How to Use Component Variables
The 'placeholder' text in the yaml should have you desired key name in all caps with `###ZARF_` prepended and `###` appened at the end.

For example, if I wanted to create a template for a database username I would do something like `###ZARF_DATABASE_USERNAME###` in the yaml.

In the zarf.yaml you would add the key-value pair in the variables section. For that same example as above, I would have (note that we don't include the `###ZARF_` and `###` and the beginning and end):
```yaml
variables:
  database_username: "iamdata"
```
