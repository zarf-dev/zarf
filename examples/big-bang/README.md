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

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />

:::caution

`valuesFiles` are processed in the order provided with Zarf adding an initial values file to populate registry and git server credentials as the first file.  Including credential `values` (even empty ones) will override these values.  This can be used to our advantage however for things like YOLO mode as described below.

:::

## Big Bang YOLO Mode Support

The Big Bang extension also supports YOLO mode, provided that you add your own credentials for the image registry. This is accomplished below with the `provision-flux-credentials` component and the `credentials.yaml` values file which allows images to be pulled from [registry1.dso.mil](https://registry1.dso.mil). We demonstrate providing account credentials via Zarf Variables, but there are other ways to populate the data in `private-registry.yaml`.

You can learn about YOLO mode in the [FAQ](../../docs/8-faq.md#what-is-yolo-mode-and-why-would-i-use-it) or the [YOLO mode example](../yolo/README.md).

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder, then select the `yolo` folder.

:::

<ExampleYAML src={require('./yolo/zarf.yaml')} showLink={false} />
