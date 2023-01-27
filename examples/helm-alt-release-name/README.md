# Helm Alt Release Name

This example shows how you can specify an alternate release name using the `releaseName` within a components `charts`.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

``` yaml
components:
  - name: component-name
    charts:
      - name: chart-name
        releaseName: alt-release-name
```
