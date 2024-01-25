import Properties from '@site/src/components/SchemaItemProperties';
import ExampleYAML from '@site/src/components/ExampleYAML';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Package Components

:::note

The following examples are not all-inclusive and are only meant to showcase the different types of resources that can be defined in a component. For a full list of fields and their options, please see the [component schema documentation](4-zarf-schema.md#components).

:::

## Overview

The actual capabilities that Zarf Packages provided are defined within named components.

These components define what dependencies they have along with a declarative definition of how they should be deployed.

Each package can have as many components as the package creator wants but a package isn't anything without at least one component.

Fully defined examples of components can be found in the [examples section](/examples/) of the documentation.

## Common Component Fields

There are certain fields that will be common across all component definitions. These fields are:

<Properties item="ZarfComponent" invert include={["files","charts","manifests","images","repos","dataInjections","extensions","scripts","actions"]} />

### Actions

<Properties item="ZarfComponent" include={["actions"]} />

Component actions are explored in the [component actions documentation](7-component-actions.md).

### Files

<Properties item="ZarfComponent" include={["files"]} />

Files can be:

- Relative paths to either a file or directory (from the `zarf.yaml` file)
- A remote URL (http/https)
- Verified using the `shasum` field for data integrity (optional and only available for files)

#### File Examples

<Tabs queryString="file-examples">
<TabItem value="Local">
<ExampleYAML src={require('../../examples/component-actions/zarf.yaml')} component="on-deploy-with-template-use-of-variable" />
</TabItem>
<TabItem value="Remote with SHA sums">
<ExampleYAML src={require('../../packages/distros/k3s/zarf.yaml')} component="k3s" />
</TabItem>
</Tabs>

### Helm Charts

<Properties item="ZarfComponent" include={["charts"]} />

Charts using the `localPath` key can be:

- Relative paths to either a file or directory (from the `zarf.yaml` file)

Charts using the `url` key can be:

- A remote URL (http/https) to a Git repository
- A remote URL (oci://) to an OCI registry
- A remote URL (http/https) to a Helm repository

:::note

To use a private Helm repository the repo must be added to Helm. You can add a repo to Helm with the [`helm repo add`](https://helm.sh/docs/helm/helm_repo_add/) command or the internal [`zarf tools helm repo add`](../2-the-zarf-cli/100-cli-commands/zarf_tools_helm_repo_add.md) command.

:::

#### Chart Examples

<ExampleYAML src={require('../../examples/helm-charts/zarf.yaml')} component="demo-helm-charts" />

### Kubernetes Manifests

<Properties item="ZarfComponent" include={["manifests"]} />

Manifests under the `files` key can be:

- Relative paths to a Kubernetes manifest file (from the `zarf.yaml` file)
- Verified using the `url@shasum` syntax for data integrity (optional and only for remote URLs)

Manifests under the `kustomizations` key can be:

- Any valid Kustomize reference both local and [remote](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/remoteBuild.md) (ie. anything you could do a `kustomize build` on)

:::note

Zarf dynamically generates a Helm Chart from the named manifest entries that you specify.  This means that any given set of files under a manifest entry will be applied according to [Helm Chart template and manifest install ordering](https://github.com/helm/helm/blob/main/pkg/releaseutil/manifest_sorter.go#L78) and not necessarily in the order that files are declared.  If ordering is important, consider moving each file into its own manifest entry in the `manifests` array.

:::

#### Manifest Examples

<Tabs queryString="manifest-examples">
<TabItem value="Local">
<ExampleYAML src={require('../../examples/manifests/zarf.yaml')} component="httpd-local" />
</TabItem>
<TabItem value="Remote">
<ExampleYAML src={require('../../examples/manifests/zarf.yaml')} component="nginx-remote" />
</TabItem>
<TabItem value="Kustomizations">

:::info

Kustomizations are handled a bit differently than normal manifests in that Zarf will automatically run `kustomize build` on them during `zarf package create`, thus rendering the Kustomization into a single manifest file.  This prevents needing to grab any remote Kustomization resources during `zarf package deploy` but also means that any Zarf [`variables`](../../examples/variables/README.md#deploy-time-variables-and-constants) will only apply to the rendered manifest not the `kustomize build` process.

:::

<ExampleYAML src={require('../../examples/manifests/zarf.yaml')} component="podinfo-kustomize" />
</TabItem>
</Tabs>

### Container Images

<Properties item="ZarfComponent" include={["images"]} />

Images can either be discovered manually, or automatically by using [`zarf dev find-images`](../2-the-zarf-cli/100-cli-commands/zarf_dev_find-images.md).

:::note

`zarf dev find-images` will find images for most standard manifests, kustomizations, and helm charts, however some images cannot be discovered this way as some upstream resources (like operators) may bury image definitions inside.  For these images, `zarf dev find-images` also offers support for the draft [Helm Improvement Proposal 15](https://github.com/helm/community/blob/main/hips/hip-0015.md) which allows chart creators to annotate any hidden images in their charts along with the [values conditions](https://github.com/helm/community/issues/277) that will cause those images to be used.

:::

#### Image Examples

<ExampleYAML src={require('../../examples/podinfo-flux/zarf.yaml')} component="flux" />

### Git Repositories

The [`git-data`](/examples/git-data/) example provides an in-depth explanation of how to include Git repositories in your Zarf package to be pushed to the internal/external Git server.

The [`podinfo-flux`](/examples/podinfo-flux/) example showcases a simple GitOps workflow using Flux and Zarf.

<Properties item="ZarfComponent" include={["repos"]} />

#### Repository Examples

<Tabs queryString="git-repo-examples">
<TabItem value="Full Mirror">
<ExampleYAML src={require('../../examples/git-data/zarf.yaml')} component="full-repo" />
</TabItem>
<TabItem value="Specific Tag">
<ExampleYAML src={require('../../examples/git-data/zarf.yaml')} component="specific-tag" />
</TabItem>
<TabItem value="Specific Branch">
<ExampleYAML src={require('../../examples/git-data/zarf.yaml')} component="specific-branch" />
</TabItem>
<TabItem value="Specific Hash">
<ExampleYAML src={require('../../examples/git-data/zarf.yaml')} component="specific-hash" />
</TabItem>
</Tabs>

### Data Injections

<Properties item="ZarfComponent" include={["dataInjections"]} />

<ExampleYAML src={require('../../examples/kiwix/zarf.yaml')} component="kiwix-serve" />

### Component Imports

<Properties item="ZarfComponent" include={["import"]} />

<Tabs queryString="import-examples">
<TabItem value="Local Path">
<ExampleYAML src={require('../../examples/composable-packages/zarf.yaml')} component="local-games-path" />
</TabItem>
<TabItem value="OCI URL">
<ExampleYAML src={require('../../examples/composable-packages/zarf.yaml')} component="oci-games-url" />
</TabItem>
</Tabs>

:::note

During composition, Zarf will merge the imported component with the component that is importing it. This means that if the importing component defines a field that the imported component also defines, the value from the importing component will be used and override.

This process will also merge `variables` and `constants` defined in the imported component's `zarf.yaml` with the importing component. The same override rules apply here as well.

:::

### Extensions

<Properties item="ZarfComponent" include={["extensions"]} />

<ExampleYAML src={require('../../examples/big-bang/zarf.yaml')} component="bigbang" />

## Deploying Components

When deploying a Zarf package, components are deployed in the order they are defined in the `zarf.yaml`.

The `zarf.yaml` configuration for each component also defines whether the component is 'required' or not. 'Required' components are always deployed without any additional user interaction while optional components are printed out in an interactive prompt asking the user if they wish to the deploy the component.

If you already know which components you want to deploy, you can do so without getting prompted by passing the components as a comma-separated list to the `--components` flag during the deploy command.

```bash
# deploy all required components, prompting for optional components and variables
$ zarf package deploy ./path/to/package.tar.zst

# deploy all required components, ignoring optional components and variable prompts
$ zarf package deploy ./path/to/package.tar.zst --confirm

# deploy optional-component-1 and optional-component-2 components whether they are required or not
$ zarf package deploy ./path/to/package.tar.zst --components=optional-component-1,optional-component-2
```

:::tip

You can deploy components in a package using globbing as well.  The following would deploy all components regardless of optional status:

```bash
# deploy optional-component-1 and optional-component-2 components whether they are required or not
$ zarf package deploy ./path/to/package.tar.zst --components=*
```

If you have any `default` components in a package definition you can also exclude those from the CLI with a leading dash (`-`) (similar to how you can exclude search terms in a search engine).

```bash
# deploy optional-component-1 but exclude default-component-1
$ zarf package deploy ./path/to/package.tar.zst --components=optional-component-1,-default-component-1
```

:::
