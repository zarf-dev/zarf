This example imports the same Nginx component locally and from an OCI registry. The component provides a Helm chart with a Deployment, Service, and ConfigMap. The consuming package selects a separate namespace for each import and configures its connection name and greeting through `values.yaml`.

This is useful if you want to slightly tweak a given component while maintaining a common core.

To learn more, see [Component Imports](/ref/components/#component-imports).
