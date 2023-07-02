import ExampleYAML from "@site/src/components/ExampleYAML";

# Manifests

This example shows you how to specify Kubernetes resources in a component's `manifests` list.  These files can either be local or remote and under the hood Zarf will wrap them in an auto-generated helm chart to manage their install, rollback, and uninstall logic.

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="manifests" showLink={false} />
