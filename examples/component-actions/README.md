import ExampleYAML from '@site/src/components/ExampleYAML';

# Component Actions

:::note

Component Actions have replaced Component Scripts. Zarf will still read scripts entries, but will convert them to actions. Component Scripts will be removed in a future release. Please update your package configurations to use Component Actions instead.

:::

This example demonstrates how to define actions within your package that can run either on `zarf package create`, `zarf package deploy` or `zarf package remove`. These actions will be executed with the context that the Zarf binary is executed with.

For more details on component actions, see the [component actions](../../docs/3-create-a-zarf-package/7-component-actions.md) documentation.

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="component-actions" showLink={false} />
