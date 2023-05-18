# OCI Differential

This test package is used to test the functionality of creating a Zarf Package while using the `--differential` flag where one of the components in the package we're creating is a component that has been imported from an OCI registry.

This test package demonstrate that OCI imported components will not be included during a differential package creation and that the proper build metadata will be added to the finalized package to ensure users of the package know which OCI imported components were not included.

This test also includes components for more standard differential package creation to make sure all of that expected functionality remains the same when there are also OCI imported components in the package.
