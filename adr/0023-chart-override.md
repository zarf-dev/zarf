# 23. Simplifying Helm Chart Value Overrides

Date: 2024-03-25

## Status

Accepted


## Context

The process of deploying applications with Helm charts in Kubernetes environments often necessitates the customization of chart values to align with specific operational or environmental requirements. The current method for customizing these values—either through manual edits or `###ZARF_VAR_XYZ###`. A more streamlined approach would greatly enhance the deployment experience by offering both flexibility and reliability.

## Decision

To address this issue, we propose the introduction of a feature designed to simplify the process of overriding chart values at the time of deployment. This feature would allow users to easily specify overrides for any chart values directly via command-line arguments, eliminating the need to alter the chart's default values file or manage multiple command-line arguments for each override.

Key aspects of the proposed implementation include:
- Use existing `--set`  flags to specify overrides for chart values.
- The ability to list all overrides in a structured and easily understandable format within `zarf.yaml`.
- Ensuring that during deployment, these specified overrides take precedence over the chart's default values, thus facilitating customized deployments without necessitating permanent modifications to the chart.

## Consequences

Adopting this feature would lead to several key improvements:
- **Streamlined Configuration Process**: Allowing helm values overrides in the zarf package schema simplifies the user experience by reducing the reliance on extensive custom `###ZARF_VAR_XYZ###` templating and aligning more closely with standard Helm practices

Ultimately, this feature is aimed at enhancing the deployment workflow by offering a straightforward and efficient means of customizing Helm chart deployments via command-line inputs.

## Example Configuration

Below is an example of how the `zarf.yaml` configuration file might be structured to utilize the new override feature for Helm chart values:

```yaml
kind: ZarfPackageConfig
metadata:
  name: helm-charts
  description: Example showcasing multiple ways to deploy helm charts
  version: 0.0.1

components:
  - name: demo-helm-charts
    required: true
    charts:
      - name: podinfo-local
        version: 6.4.0
        namespace: podinfo-from-local-chart
        localPath: chart
        valuesFiles:
          - values.yaml
        variables:
          - name: REPLICA_COUNT
            description: "Override the number of pod replicas"
            path: replicaCount
```
This configuration allows for the specification of default values and descriptions for variables that can be overridden at deployment time. The variables section under each chart specifies the variables that can be overridden, along with a path that indicates where in the values file the variable is located.

### Command Line Example

To override the `REPLICA_COUNT` variable at deployment time, the following command can be used:

```bash
zarf package deploy zarf-package-helm-charts-arm64-0.0.1.tar.zst --set REPLICA_COUNT=5
```
This command demonstrates how users can easily customize their Helm chart deployments by specifying overrides for chart values directly via command-line arguments, in line with the proposed feature.
