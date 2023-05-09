import ExampleYAML from "@site/src/components/ExampleYAML";

# Composable Packages

This example demonstrates using Zarf to compose existing zarf packages into another package.  It uses the existing [zarf game](../dos-games/) example by simply adding an `import` and `path` in the new [zarf.yaml](zarf.yaml).

:::note

Import paths must be statically defined at create time.  You cannot use [variables](../variables/) in them

:::

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="composable-packages" showLink={false} />
