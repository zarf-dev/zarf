import ExampleYAML from "@site/src/components/ExampleYAML";

# Podinfo via Flux

This example demonstrates how to use flux with Zarf to deploy the `stefanprodan/podinfo` app using GitOps.

It uses a vanilla configuration of flux with upstream containers.

If you want to learn more about how Zarf handles `git` repositories, see the [git-data](../git-data/) example.

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="podinfo-flux" showLink={false} />
