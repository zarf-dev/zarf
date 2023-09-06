import ExampleYAML from "@site/src/components/ExampleYAML";

# Composable Packages

This example demonstrates using Zarf to import components from existing Zarf package definitions while merging overrides to add or change functionality.  It uses the existing [DOS games](../dos-games/README.md) and [WordPress](../wordpress/README.md) examples by simply adding `import` keys in the new [zarf.yaml](zarf.yaml) file.

The `import` key in Zarf supports two modes to pull in a component:

1. The `path` key allows you to specify a path to a directory that contains the `zarf.yaml` that you wish to import on your local filesystem.  This allows you to have a common component that you can reuse across multiple packages *within* a project.

2. The `url` key allows you to specify an `oci://` URL to a skeleton package that was published to an OCI registry.  Skeleton packages are special package bundles that contain the `zarf.yaml` package definition and any local files referenced by that definition at publish time.  This allows you to version a set of components and import them into multiple packages *across* projects.

:::tip

You can create a skeleton package from a `zarf.yaml` by pointing `zarf package publish` at the directory that contains it:

```bash
zarf package publish path/containing/package/definition oci://your-registry.com
```

:::

## Merge Strategies

When merging components together Zarf will adopt the following strategies depending on the kind of primitive (`files`, `required`, `manifests`) that it is merging:

| Kind                       | Key(s)                                 | Description |
|----------------------------|----------------------------------------|-------------|
| Component Behavior         | `name`, `group`, `default`, `required` | These keys control how Zarf interacts with a given component and will _always_ take the value of the overriding component |
| Component Description      | `description` | This key will only take the value of the overriding component if it is not empty |
| Cosign Key Path            | `cosignKeyPath` | [Deprecated] This key will only take the value of the overriding component if it is not empty |
| Un'name'd Primitive Arrays | `actions`, `dataInjections`, `files`, `images`, `repos` | These keys will append the overriding component's version of the array to the end of the base component's array |
| 'name'd Primitive Arrays   | `charts`, `manifests` | For any given element in the overriding component, if the element matches based on `name` then its values will be merged with the base element of the same `name`. If not then the element will be appended to the end of the array |

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

:::note

Creating this example requires a locally hosted container registry that has the `wordpress` skeleton package published and available. You can do this by running the following commands:

```bash
docker run -d -p 555:5000 --restart=always --name registry registry:2
zarf package publish examples/wordpress oci://127.0.0.1:555 --insecure
```

You will also need to pass the `--insecure` flag to `zarf package create` to pull from the `http` registry:

```bash
zarf package create examples/composable-packages/ --insecure
```

:::

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />

:::info

As you can see in the example, the `import` key can be combined with other keys to merge components together.  This can be done as many components deep as you wish and in the end will generate one main `zarf.yaml` file with all of the defined resources included.

This is useful if you want to slightly tweak a given component while maintaining a common core.

:::

:::note

The import `path` or `url` must be statically defined at create time.  You cannot use [package templates](../variables/README.md#create-time-package-configuration-templates) within them.

:::
