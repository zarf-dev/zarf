import ExampleYAML from '@site/src/components/ExampleYAML';

# Big Bang

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using the Zarf `bigbang` extension.

The `bigbang` noun sits within the `extensions` specification of Zarf and provides the following configuration:

- `version`     - The version of Big Bang to use
- `repo`        - Override repo to pull Big Bang from instead of Repo One
- `skipFlux`    - Whether to skip deploying flux; Defaults to false
- `valuesFiles` - The list of values files to pass to Big Bang; these will be merged together

To see a tutorial for the creation and deployment of this package see the [Big Bang Tutorial](../../docs/5-zarf-tutorials/6-big-bang.md).

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="big-bang" showLink={false} />

## YOLO Mode

You can learn about YOLO mode [here](https://docs.zarf.dev/docs/faq#what-is-yolo-mode-and-why-would-i-use-it).

The `provision-flux-credentials` component is required to create the necessary secret to pull flux images from [registry1.dso.mil](https://registry1.dso.mil). In the provided `zarf.yaml` for this example, we demonstrate providing account credentials via Zarf Variables, although there are other ways to populate the data in `private-registry.yaml`.

### Big Bang YOLO `zarf.yaml`

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder, then select the `yolo` folder.

:::

<ExampleYAML example="big-bang/yolo" showLink={false} />
