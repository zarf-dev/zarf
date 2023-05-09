import ExampleYAML from '@site/src/components/ExampleYAML';

# Big Bang

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using the Zarf `bigbang` extension.

The `bigbang` noun sits within the `extensions` specification of Zarf and provides the following configuration:

- `version`     - The version of Big Bang to use
- `repo`        - Override repo to pull Big Bang from instead of Repo One
- `skipFlux`    - Whether to skip deploying flux; Defaults to false
- `valuesFiles` - The list of values files to pass to Big Bang; these will be merged together

To see a tutorial for the creation and deployment of this package see the [Big Bang Tutorial](../../docs/6-zarf-tutorials/6-big-bang.md).

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="big-bang" showLink={false} />
