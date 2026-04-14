:::caution

Component Actions have replaced Component Scripts. Zarf will still read `scripts` entries, but will convert them to `actions`. Component Scripts will be removed in a future release. Please update your package configurations to use Component Actions instead.

:::

This example demonstrates how to define actions within your package that can run either on `zarf package create`, `zarf package deploy` or `zarf package remove`. These actions will be executed with the context that the Zarf binary is executed with.

For more details on component actions, see the [component actions](/ref/actions/) documentation.
