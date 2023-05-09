import Properties from '@site/src/components/SchemaItemProperties';
import ExampleYAML from '@site/src/components/ExampleYAML';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Package Components

:::warning

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

Component actions are explored in the [component actions documentation](6-component-actions.md).

### Files

<Properties item="ZarfComponent" include={["files"]} />

Can be:

- Relative paths to either a file or directory (from the `zarf.yaml` file)
- A remote URL (http/https)
- Verified using the `shasum` field for data integrity (optional and only available for files)

#### File Examples

<Tabs queryString="file-examples">
<TabItem value="Local and Remote">
<ExampleYAML example="terraform" component="download-terraform" />
</TabItem>
<TabItem value="Remote with SHA sums">

```yaml title="packages/distros/k3s/zarf.yaml"
  - name: k3s
    import:
      path: common
      name: k3s
    only:
      cluster:
        architecture: amd64
    files:
      # Include the actual K3s binary
      - source: https://github.com/k3s-io/k3s/releases/download/v1.24.1+k3s1/k3s
        shasum: ca398d83fee8f9f52b05fb184582054be3c0285a1b9e8fb5c7305c7b9a91448a
        target: /usr/sbin/k3s
        executable: true
        # K3s magic provides these tools when symlinking
        symlinks:
          - /usr/sbin/kubectl
          - /usr/sbin/ctr
          - /usr/sbin/crictl
      # Transfer the K3s images for containerd to pick them up
      - source: https://github.com/k3s-io/k3s/releases/download/v1.24.1+k3s1/k3s-airgap-images-amd64.tar.zst
        shasum: 6736f9fa4d5754d60b0508bafb2f888170cb99a2d93a3a1617a919ca4ee74034
        target: /var/lib/rancher/k3s/agent/images/k3s.tar.zst
    actions:
      onDeploy:
        before:
          - cmd: if [ "$(arch)" != "x86_64" ]; then echo "this package architecture is amd64, but the target system has a different architecture. These architectures must be the same" && exit 1; fi
            description: Check that the host architecture matches the package architecture
            maxRetries: 0
```

</TabItem>
</Tabs>

### Helm Charts

<Properties item="ZarfComponent" include={["charts"]} />

Can be when using the `localPath` key:

- Relative paths to either a file or directory (from the `zarf.yaml` file)

Can be when using the `url` key:

- A remote URL (http/https) to a Git repository
- A remote URL (oci://) to an OCI registry

#### Chart Examples

<Tabs queryString="chart-examples">
<TabItem value="localPath">
<ExampleYAML example="helm-local-chart" component="demo-helm-local-chart" />
</TabItem>
<TabItem value="URL (git)">
<ExampleYAML example="helm-git-chart" component="demo-helm-git-chart" />
</TabItem>
<TabItem value="URL (oci)">
<ExampleYAML example="helm-oci-chart" component="helm-oci-chart" />
</TabItem>
</Tabs>

### Kubernetes Manifests

<Properties item="ZarfComponent" include={["manifests"]} />

Can be when using the `files` key:

- Relative paths to a Kubernetes manifest file (from the `zarf.yaml` file)
- Verified using the `url@shasum` syntax for data integrity (optional and only for remote URLs)

Can be when using the `kustomizations` key:

- Any valid Kustomize reference both local and [remote](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/remoteBuild.md) (ie. anything you could do a `kustomize build` on)

#### Manifest Examples

<Tabs queryString="manifest-examples">
<TabItem value="Local">

> While this explanation does not showcase it, you can also specify a local directory containing a `kustomization.yaml` file and Zarf will automatically run `kustomize build` on the directory during `zarf package create`, rendering the Kustomization into a single manifest file.

<ExampleYAML example="dos-games" component="baseline" />
</TabItem>
<TabItem value="Remote">
<ExampleYAML example="remote-manifests" component="remote-manifests-and-kustomizations" />
</TabItem>
</Tabs>

### Container Images

<Properties item="ZarfComponent" include={["images"]} />

Images can either be discovered manually, or automatically by using [`zarf prepare find-images`](../2-the-zarf-cli/100-cli-commands/zarf_prepare_find-images.md).

:::note

`zarf prepare find-images` has some known limitations due to the numerous ways images can be defined in Kubernetes resources, but should work for most standard manifests, kustomizations, and helm charts.

:::

#### Image Examples

<ExampleYAML example="podinfo-flux" component="flux" />

### Git Repositories

The [`git-data`](/examples/git-data/) example provides an in-depth explanation of how to include Git repositories in your Zarf package to be pushed to the internal/external Git server.

The [`podinfo-flux`](/examples/podinfo-flux/) example showcases a simple GitOps workflow using Flux and Zarf.

<Properties item="ZarfComponent" include={["repos"]} />

#### Repository Examples

<Tabs queryString="git-repo-examples">
<TabItem value="Full Mirror">
<ExampleYAML example="git-data" component="full-repo" />
</TabItem>
<TabItem value="Specific Tag">
<ExampleYAML example="git-data" component="specific-tag" />
</TabItem>
<TabItem value="Specific Branch">
<ExampleYAML example="git-data" component="specific-branch" />
</TabItem>
<TabItem value="Specific Hash">
<ExampleYAML example="git-data" component="specific-hash" />
</TabItem>
</Tabs>

### Data Injections

<Properties item="ZarfComponent" include={["dataInjections"]} />

<ExampleYAML example="data-injection" component="with-init-container" />

### Component Imports

<Properties item="ZarfComponent" include={["import"]} />

<Tabs queryString="import-examples">
<TabItem value="Path">
<ExampleYAML example="composable-packages" component="games" />
</TabItem>
<TabItem value="OCI">
<ExampleYAML example="composable-packages" component="chart-via-oci" />
</TabItem>
</Tabs>

:::note

During composition, Zarf will merge the imported component with the component that is importing it. This means that if the importing component defines a field that the imported component also defines, the value from the importing component will be used and override.

This process will also merge `variables` and `constants` defined in the imported component's `zarf.yaml` with the importing component. The same ovveride rules apply here as well.

:::

### Extensions

<Properties item="ZarfComponent" include={["extensions"]} />

<ExampleYAML example="big-bang" component="bigbang" />

## Deploying Components

When deploying a Zarf package, compone are deployed in the order they are defined in the `zarf.yaml`.

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
