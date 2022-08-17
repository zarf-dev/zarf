# Data Injection

This example shows Zarf's ability to inject data into a container running in a pod.  This allows for data that is not included in the container image to be injected at deploy time.

Data injections are declared using the `dataInjections` key within a component, and once the specified container is started, Zarf will copy the files and folders from the specified source into the specified container and path.

```
dataInjections:
  - source: path-to/pull-from
    target:
      namespace: target-namespace
      selector: my-label=the-selected-label
      container: container-to-inject-into
      path: /path/inside-the/container
    compress: true # whether to compress the injection stream (requires gzip)
```

> ⚠️ **NOTE:** *The source should be defined relative to the component's package*
