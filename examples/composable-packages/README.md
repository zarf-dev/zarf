import ExampleYAML from "@site/src/components/ExampleYAML";

# Composable Packages

This example demonstrates using Zarf to compose existing zarf packages into another package.  It uses the existing [zarf game](../dos-games/) example by simply adding an `import` and `path` in the new [zarf.yaml](zarf.yaml).

## Example Prerequisites

Creating this example requires a locally hosted container registry that has the `helm-charts` skeleton package published and available. You can do this by running the following commands:

```bash
docker run -d -p 5000:5000 --restart=always --name registry registry:2
zarf package publish examples/helm-charts oci://127.0.0.1:5000 --insecure
```

:::note

Import paths must be statically defined at create time.  You cannot use [variables](../variables/) in them.

:::

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="composable-packages" showLink={false} />
