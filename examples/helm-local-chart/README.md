# Helm Local Chart
This example shows how you can specify a local chart for a helm source within a component's `charts`.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

``` yaml
components:
  - name: component-name
    charts:
      - name: chart-name
        localPath: path/to/chart
```
