# 23. Simplifying Helm Chart Value Overrides

Date: 2024-03-25

## Status

Proposed


## Context

The process of deploying applications with Helm charts in Kubernetes environments often necessitates the customization of chart values to align with specific operational or environmental requirements. The current method for customizing these values—either through manual edits or `###`. A more streamlined approach would greatly enhance the deployment experience by offering both flexibility and reliability.

## Decision

To address this issue, we propose the introduction of a feature designed to simplify the process of overriding chart values at the time of deployment. This feature would allow users to easily specify overrides for any chart values directly via command-line arguments, eliminating the need to alter the chart's default values file or manage multiple command-line arguments for each override.

Key aspects of the proposed implementation include:
- Use existing `--set`  flags to specify overrides for chart values.
- The ability to list all overrides in a structured and easily understandable format within the ZarfConfig file.
- Ensuring that during deployment, these specified overrides take precedence over the chart's default values, thus facilitating customized deployments without necessitating permanent modifications to the chart.

## Consequences

Adopting this feature would lead to several key improvements:
- **Streamlined Configuration Process**: Centralizing overrides in a single, unified file significantly simplifies the management of configuration settings, aligning more closely with standard Helm practices and reducing the reliance on extensive custom `###` templating.

Ultimately, this feature is aimed at enhancing the deployment workflow by offering a straightforward and efficient means of customizing Helm chart deployments via command-line inputs.
