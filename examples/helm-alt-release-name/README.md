# Helm Alt Release Name

This example shows how you can specify an alternate release name using the `releaseName` within a components `charts`.

[Full Example](https://github.com/defenseunicorns/zarf/tree/master/examples/helm-alt-release-name)

```
components:
  - name: component-name
    charts:
      - name: chart-name
        releaseName: alt-release-name
```
