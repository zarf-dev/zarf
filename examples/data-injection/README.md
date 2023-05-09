import ExampleYAML from "@site/src/components/ExampleYAML";

# Data Injection

This example shows Zarf's ability to inject data into a container running in a pod.  This allows for data that is not included in the container image to be injected at deploy time.

Data injections are declared using the `dataInjections` key within a component, and once the specified container is started, Zarf will copy the files and folders from the specified source into the specified container and path.

:::caution

This feature depends on the `tar` (and for `compress`, `gzip`) executables and their implementation across operating systems.  Between macOS and Linux there is general agreement on how these utilities should function, however on Windows you may see issues enabling compression.

To resolve this you can either disable compression or use the GNU core-utils version of `tar` and `gzip`.

:::

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="data-injection" showLink={false} />
