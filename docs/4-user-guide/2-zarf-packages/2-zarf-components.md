---
sidebar_position: 2
---

# Understanding Zarf Components

The capabilities that Zarf Packages provide are defined within named components. These components define their required dependencies and provide a declarative definition for how they should be deployed. Although a package creator may include multiple components in their package, it is essential to have at least one component for a package to exist.

Components can define a wide range of resources necessary for package deployment. The schema for the components is accessible via the [Zarf Component Schema](../3-zarf-schema.md#components) page. Some examples of the types of resources that a component can define are:

* Files to move onto the host.
* Helm charts to install into the running K8s cluster.
* Raw Kubernetes manifests to deploy (by getting converted into zarf-generated helm charts and installed).
* Container images to push into the registry the init-package created in the K8s cluster.
* Git repositories to push into the git server the init-package created in the K8s cluster.
* Data to push into a resource (i.e. a pod) in the K8s cluster.
* Scripts to run before/after the component is deployed.

### Deploying a Component

When you deploy a Zarf Package, the **components inside it are deployed in the order specified in the `zarf.yaml` file used to create the package.** Each component in the `zarf.yaml` configuration file is marked as either required or optional. Required components are deployed automatically without any user input when the package is deployed. Optional components are printed out in an interactive prompt, allowing you to choose whether or not to deploy them.

If you know which components you want to deploy, you can avoid getting prompted by specifying them as a comma-separated list to the  `--components` flag when executing the deploy command. For instance, you can enter `zarf package deploy ./path/to/package.tar.zst --components=optional-component-1,optional-component-2` to deploy those specific components.

## Composing Package Components

Existing components from other packages can be composed in new packages. This can be achieved by using the import field and providing a path to the `zarf.yaml` you wish to compose.

```yaml
components:
  - name: flux
    import:
     path: 'path/to/flux/package/directory/'
```

If you don't specify the component name in the import field, Zarf will attempt to import a component with the same name as the one currently being defined from the specified path. In the example above, since the new component is called 'flux', Zarf will import the 'flux' component from the specified path. However, if you plan to give the new component a different name, you can specify the name of the package that needs to be imported in the import field.

```yaml
components:
  - name: flux
    import:
     path: 'path/to/flux/package/directory/'
     name: flux-v1.0.0
```

:::note
When importing a component, Zarf will copy all of the values from the original component except for the `required` key. In addition, while Zarf will copy the values, you have the ability to override the value for the `description` key.

See the [composable-packages](https://github.com/defenseunicorns/zarf/blob/master/examples/composable-packages/zarf.yaml) example to see this in action.
:::


## What Makes up a Component

Zarf components can contain different key/value pairs, which you can learn more about under the `components` section on the [Zarf Component Schema](../3-zarf-schema.md#components) page.
