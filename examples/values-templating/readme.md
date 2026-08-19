# [BETA] Values & Templates Example

This example demonstrates Zarf's beta values templating system, including support for **Sprig functions** for advanced template processing, **Helm chart value overrides**, and **cluster state access** via `.State`.

## Features Demonstrated

- **Basic templating** with `{{ .Values.* }}`, `{{ .Build.* }}`, `{{ .Metadata.* }}`, `{{ .Constants.* }}`, and `{{ .Variables.* }}`
- **Cluster state** with `{{ .State.Registry.Address }}`, `{{ .State.StorageClass }}`, `{{ .State.IPFamily }}`, and other non-sensitive runtime fields via `.State`
- **Package object** access via `.Pkg`, including `{{ (.Pkg.GetComponent "name").Images }}` to look up a component by name
- **Sprig functions** for string manipulation, lists, math, encoding, and more
- **File templating** with both simple substitution and complex transformations
- **Dynamic configuration** using template functions for practical Kubernetes deployments
- **Helm chart value overrides** mapping Zarf values to Helm chart values

## Sprig Functions Showcased

The example includes demonstrations of popular Sprig functions:
- **String functions**: `upper`, `lower`, `title`, `kebabcase`, `snakecase`, `quote`
- **List functions**: `join`, `len`, `first`, `last`, `sortAlpha`, `reverse`
- **Default functions**: `default` for fallback values
- **Math functions**: `add`, `mul`, `max`, `min`
- **Encoding functions**: `b64enc`, `sha256sum`
- **Utility functions**: `repeat`, `indent`, `trunc`, `toString`

## Try It Out

Deploy this example to see values and templates in action:

```bash
# Optional: Inspect the manifests and chart values-files
zarf dev inspect manifests
zarf dev inspect values-files

# Create and deploy the package
zarf package create . --confirm
zarf package deploy zarf-package-values-templating-*.tar.zst --confirm

# View the nginx results
kubectl get configmap nginx-configmap -n nginx -o yaml
zarf connect nginx

# View the helm chart results
kubectl get configmap -n helm-overrides -o yaml

# Remove the package with values templating in remove actions
# Feel free to change --set-values to whatever you want!
zarf package remove values-templating --confirm --set-values="site.name=Example,app.environment=test,site.organization=ZarfDev"
```
