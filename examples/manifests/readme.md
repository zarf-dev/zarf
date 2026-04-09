This example shows you how to specify Kubernetes resources in a component's `manifests` list.  These files can either be local or remote and under the hood Zarf will wrap them in an auto-generated helm chart to manage their install, rollback, and uninstall logic.

To learn more about how `manifests` work in Zarf, see the [Kubernetes Manifests section](/ref/components/#kubernetes-manifests) of the package components documentation.
