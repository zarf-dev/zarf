import ExampleYAML from "@site/src/components/ExampleYAML";

# Zarf Agent Metrics (Prometheus)

This example demonstrates how to scrape the Zarf Agent container image from the Prometheus Operator.

## Prerequisites

- A running K8s cluster.

:::note

The cluster does not need to have the Zarf init package installed or any other Zarf-related bootstrapping.

:::

## Instructions

Initialize Zarf (interactively):

```bash
zarf init
# Make these choices at the prompts
# ? Do you want to download this init package? Yes
# ? Deploy this Zarf package? Yes
# ? Deploy the k3s component? No
# ? Deploy the logging component? No
# ? Deploy the git-server component? No
```

Create the package:

```bash
zarf package create --confirm
```

Deploy the package

```bash
# Run the following command to deploy the created package to the cluster
zarf package deploy

# Choose the yolo package from the list
? Choose or type the package file [tab for suggestions]
> zarf-package-scrape-zarf-agent-<ARCH>.tar.zst

# Confirm the deployment
? Deploy this Zarf package? (y/N) [y]
```

Port-forward the Prometheus Operator's Prometheus instance in the Prometheus UI ,then click `Status` => `Targets`. Scroll down to find `serviceMonitor/monitoring/monitoring-agent-hook`:

```bash
zarf connect prometheus-operated-agent
```


## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="scraping-zarf-agent" showLink={false} />
