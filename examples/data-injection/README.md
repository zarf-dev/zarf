# Data Injection

This example shows Zarf's ability to inject data into a container running in a pod.  This allows for data that is not included in the container image to be injected at deploy time.

Data injections are declared using the `dataInjections` key within a component, and once the specified container is started, Zarf will copy the files and folders from the specified source into the specified container and path.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

``` yaml
dataInjections:
  - source: path-to/pull-from
    target:
      namespace: target-namespace
      selector: my-label=the-selected-label
      container: container-to-inject-into
      path: /path/inside-the/container
    compress: true # whether to compress the injection stream (requires gzip)
```

:::note

The source should be defined relative to the component's package.

:::

:::caution

This feature depends on the `tar` (and for `compress`, `gzip`) executables and their implementation across operating systems.  Between macOS and Linux there is general agreement on how these utilities should function, however on Windows you may see issues enabling compression.

To resolve this you can either disable compression or use the GNU core-utils version of `tar` and `gzip`.

:::
