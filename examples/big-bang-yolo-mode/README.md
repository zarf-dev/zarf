import ExampleYAML from '@site/src/components/ExampleYAML';

# Big Bang (YOLO Mode)

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using the Zarf `bigbang` extension with YOLO mode enabled. You can learn about YOLO mode [here](https://docs.zarf.dev/docs/faq#what-is-yolo-mode-and-why-would-i-use-it).

The `provision-flux-credentials` component is required to create the necessary secret to pull flux images from [registry1.dso.mil](https://registry1.dso.mil). In the provided `zarf.yaml` for this example, we demonstrate providing account credentials via Zarf Variables, although there are other ways to populate the data in `private-registry.yaml`.

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="big-bang-yolo-mode" showLink={false} />
