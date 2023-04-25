# Helm Git Chart
This example shows how you can specify a Git repository chart for a helm source within a component's `charts`.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

``` yaml
components:
  - name: component-name
    charts:
      - name: chart-name
        url: url-to-git-repo.git
        gitPath: path/to/chart/in/repo
```
