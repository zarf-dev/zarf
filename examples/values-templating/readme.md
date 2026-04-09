# [ALPHA] Values & Templates Example

This example demonstrates the pre-release alpha version of Zarf's values templating system, including support for **Sprig functions** for advanced template processing and **Helm chart value overrides**.

## Features Demonstrated

- **Basic templating** with `{{ .Values.* }}`, `{{ .Build.* }}`, `{{ .Metadata.* }}`, `{{ .Constants.* }}`, and `{{ .Variables.* }}`
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
# Optional: Inspect the manifests and chart values-files (features="values=true" flag required until general release of values)
zarf dev inspect manifests --features="values=true"
zarf dev inspect values-files --features="values=true"

# Create and deploy the package
zarf package create . --confirm --features="values=true"
zarf package deploy zarf-package-values-templating-*.tar.zst --confirm --features="values=true"

# View the nginx results
kubectl get configmap nginx-configmap -n nginx -o yaml
zarf connect nginx

# View the helm chart results
kubectl get configmap -n helm-overrides -o yaml

# Remove the package with values templating in remove actions
# Feel free to change --set-values to whatever you want!
zarf package remove values-templating --confirm --features="values=true" --set-values="site.name=Example,app.environment=test,site.organization=ZarfDev"
```
