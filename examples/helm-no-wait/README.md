# Helm Alt Release Name

This example shows how you can specify for zarf to not wait for resources to report ready within a component's `manifests`. This is also applicable to `charts`.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

```
components:
  - name: component-name
    manifests:
      - name: chart-name
        wait: false
```
