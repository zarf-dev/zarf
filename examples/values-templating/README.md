# [ALPHA] Values & Templates Example

This example demonstrates the pre-release alpha version Zarf's values templating system, including support for **Sprig functions** for advanced template processing.

## Features Demonstrated

- **Basic templating** with `{{ .Values.* }}`, `{{ .Constants.* }}`, and `{{ .Variables.* }}`
- **Sprig functions** for string manipulation, lists, math, encoding, and more
- **Dynamic configuration** using template functions for practical Kubernetes deployments
- **File templating** with both simple substitution and complex transformations

## Sprig Functions Showcased

The example includes demonstrations of popular Sprig functions:
- **String functions**: `upper`, `lower`, `title`, `kebabcase`, `snakecase`, `quote`
- **List functions**: `join`, `len`, `first`, `last`, `sortAlpha`, `reverse`
- **Default functions**: `default` for fallback values
- **Math functions**: `add`, `mul`, `max`, `min`
- **Encoding functions**: `b64enc`, `sha256sum`
- **Utility functions**: `repeat`, `indent`, `trunc`, `toString`

## Try It Out

Deploy this example to see sprig functions in action:

```bash
# Create and deploy the package (features="values=true" flag required until general release of values)
zarf package create . --confirm --features="values=true"
zarf package deploy zarf-package-values-templating-*.tar.zst --confirm --features="values=true"

# View the results
kubectl get configmap nginx-configmap -n nginx -o yaml
zarf connect nginx
```
