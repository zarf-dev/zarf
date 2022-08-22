# Composable Packages

This example demonstrates using Zarf to compose existing zarf packages into another package.  It uses the existing [zarf game](../game/) example by simply adding an `import` and `path` in the new [zarf.yaml](zarf.yaml).

:::info

To view the example source code, select the `Edit this page` link below the article.

:::

```
components:
  - name: composed
    import:
      path: ../your-path
      name: sub-component-name
```

:::note

Import paths must be statically defined at create time.  You cannot use [package variables](../package-variables/) in them

:::
