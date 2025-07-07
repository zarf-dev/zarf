# Helm Charts Example

This example demonstrates the various ways to deploy Helm charts with Zarf, including using absolute paths for values files.

## Components

1. **demo-helm-charts**: Shows different ways to reference Helm charts:

   - Local chart from directory
   - OCI registry chart
   - Git repository chart
   - Helm repository chart

1. **demo-helm-charts-abs-path**: Demonstrates using absolute paths for values files

## Testing Absolute Path Support

The `demo-helm-charts-abs-path` component shows how to use absolute paths in `valuesFiles`. This feature is useful for:

- Sharing common values across multiple packages
- Referencing values from CI/CD mounted volumes
- Using organization-wide configuration files

### Steps to Test

1. Copy the example values file to a temporary location:

   ```bash
   cp values-override.yaml /tmp/my-values.yaml
   ```

1. Set the environment variable with the absolute path:

   ```bash
   export ZARF_VAR_VALUES_PATH=/tmp/my-values.yaml
   ```

1. Create the package:

   ```bash
   zarf package create . --confirm
   ```

1. Deploy the package:

   ```bash
   zarf package deploy zarf-package-helm-charts-*.tar.zst --confirm
   ```

1. Check that the values were applied:

   ```bash
   kubectl get pods -n podinfo-from-abs-path -o yaml | grep -A2 resources:
   ```

You should see the resource limits from the absolute path values file applied.

## Notes

- Absolute paths are preserved during package creation
- Relative paths are resolved relative to the package directory
- The `zarf dev find-images` command also supports absolute paths for values files
