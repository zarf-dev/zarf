This example shows how to reuse a web-server component through local and OCI imports. The component provides an Nginx Deployment and Service, while the consuming package selects separate namespaces and adds a ConfigMap with different web content for each import.

Manifest groups with the same name are merged: the package overrides the namespace and appends its ConfigMap to the component's manifests. Both web servers mount a ConfigMap named `web-content`, but each serves the page supplied in its own namespace.

To learn more, see [Component Imports](/ref/components/#component-imports) in the package components documentation.
