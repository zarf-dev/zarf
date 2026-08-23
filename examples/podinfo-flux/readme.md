This example demonstrates how to use Flux with Zarf to deploy the `stefanprodan/podinfo` app using GitRepositories, HelmRepositories, and OCIRepositories.

It uses a vanilla configuration of Flux with upstream containers.

To learn more about how Zarf handles `git` repositories, see the [Git Repositories section](/ref/components/#git-repositories) of the package components documentation.

:::caution

Only `type: oci` HelmRepositories are supported by the Zarf Agent. The `type` key requires a HelmRepository CRD version greater than v1beta1.

The Zarf agent will only automatically add the `insecure` key if the internal registry is used. If you are using a http registry outside of the cluster you will need to manually add this key.

:::
