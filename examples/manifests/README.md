import ExampleYAML from "@site/src/components/ExampleYAML";

# Manifests

This example shows you how to specify Kubernetes resources in a component's `manifests` list.  These files can either be local or remote and under the hood Zarf will wrap them in an auto-generated helm chart to manage their install, rollback, and uninstall logic.

To learn more about how `manifests` work in Zarf, see the [Kubernetes Manifests section](../../docs/3-create-a-zarf-package/2-zarf-components.md#kubernetes-manifests) of the package components documentation.

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />
