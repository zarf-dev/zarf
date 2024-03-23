import ExampleYAML from "@components/ExampleYAML.astro";

# Package Flavors

This example demonstrates how to define variants of packages within the same package definition.  This can be combined with [Composable Packages](../composable-packages/README.md) to build up packages and include the necessary [merge overrides](../composable-packages/README.md#merge-strategies) for each variant.

Given package flavors are built by specifying the `--flavor` flag on `zarf package create`.  This will include any components that match that flavor or that do not specify a flavor.

## `zarf.yaml` {#zarf.yaml}

:::note

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />
