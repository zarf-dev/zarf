# Component Choice

This example demonstrates how to define packages that can be chosen by the user on `zarf package deploy`.  This is done through the `group` key inside of the component specification that defines a group of components a user can select from.

A package creator can also use the `default` key to specify which component will be chosen if a user uses the `--confirm` flag.

> ⚠️ **NOTE:** *A user can only select a single component in a component group and a package creator can specify only a single default*

> ⚠️ **NOTE:** *A component in a component `group` cannot be marked as being `required`*
