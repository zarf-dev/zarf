import ExampleYAML from '@site/src/components/ExampleYAML';

# Component Webhooks

:::warn

Component Webhooks is currently an [Alpha Feature](../../docs/9-roadmap.md#alpha). This feature is not extensively tested and may be affected by breaking changes in the future. We encourage you to experiment with this feature and provide feedback to the Zarf team as we begin to stabilize this feature.

:::

This example demonstrates how to use webhooks to perform actions during the lifecycle of package deployments. Webhooks are similar to [Component Actions](../../docs/3-create-a-zarf-package/7-component-actions.md) such that they both enable complex functionality to be executed during the lifecycle of a package deployment. The key difference between webhooks and actions is that actions are defined within the package's `zarf.yaml` while webhooks are defined within the cluster that you are deploying your package onto.

This example uses `Pepr` as a mutating webhook that watches for any updates to a zarf package secret. As `Zarf` deploys components, it updates a secret in the `zarf` namespace that 'declares' what components are being deployed. `Pepr` watches for these updates and runs an example operation for each component that gets deployed to the cluster. Since `Pepr` is a mutating webhook, as `Zarf` updates the package secret for each component that is being deployed, `Pepr` will modify the secret to denote that a webhook operation is executing for that component. To account for this, every time `Zarf` updates the package secret, it will check to see if a webhook has modified the secret and will wait if there are any webhooks in a `Running` state. The webhook itself is responsible for updating the secrets when it's operation complete. `Zarf` will poll the secret every second to check if the webhook is complete and if it can continue deploying the rest of the package.


Webhooks have the potential to be extremely powerful. Since they are written in Javascript, they have the capability to do almost anything that you can do with JavaScript. This includes interacting with the Kubernetes API, interacting with other APIs, or even interacting with other systems. Caution should be exercised when deploying webhooks to clusters as they have the potential to run any time a new package is deployed to the cluster, and future package deployers might not be aware that the cluster has webhooks configured.


:::info

If you want to update the capability yourself, you will need to rebuild the `Pepr` module before creating the package.

This can be completed by running the following commands:
`npm ci`
`npx pepr build`
`zarf package create ./dist`

:::


## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />
