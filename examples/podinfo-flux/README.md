import ExampleYAML from "@site/src/components/ExampleYAML";

# Flux (with Podinfo)

This example demonstrates how to use Flux with Zarf to deploy the `stefanprodan/podinfo` app using GitRepositories, HelmRepositories, and OCIRepositories.

It uses a vanilla configuration of Flux with upstream containers.

If you want to learn more about how Zarf handles `git` repositories, see the [git-data](../git-data/README.md) example.  Zarf also supports OCI Helm Charts and OCI Flux manifests when they are included under `images` and pushed to the Zarf-managed registry.

:::caution

Only `type: oci` HelmRepositories are currently supported by the Zarf Agent, and to use them you also must have a version of Flux new enough to include the `type: oci` key.

Zarf will also inly automatically add the `insecure` key for registries on NodePort services so if you are hosting an http registry outside of the cluster you will need to manually add this key.

:::

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />
