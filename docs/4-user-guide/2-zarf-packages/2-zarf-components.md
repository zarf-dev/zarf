---
sidebar_position: 2
---

# Understanding Zarf Components

The actual capabilities that Zarf Packages provided are defined within named components. These components define what dependencies they have and a declaritive definition of how it should be deployed. Each package can have as many components as the package creator wants but a package really isn't anything without at least one component.

Components can define a wide range of resources that it needs when the package it is a part of gets deployed. The schema for the components is linked below but a high level look at the things components can define include:
 * Files to move onto the host
 * Helm charts to install into the running k8s cluster
 * Raw Kubernetes manifests to deploy (by getting converted into zarf-generated helm charts and installed)
 * Container images to push into the registry the init-package created in the k8s cluster
 * Git repositories to push into the git server the init-package created in the k8s cluster
 * Data to push into a resource (i.e. a pod) in the k8s cluster
 * Scripts to run before/after the component is deployed


### Deploying a component
When deploying a Zarf package, the **components within a package are deployed in the order they are defined in the `zarf.yaml` that the package was created from.** The `zarf.yaml` configuration for each component also defines whether the component is 'required' or not. 'Required' components are always deployed without any additional user interaction whenever the package is deployed while optional components are printed out in an interactive prompt to the user asking if they wish to the deploy the component.

 If you already know which components you want to deploy, you can do so without getting prompted by passing the components as a comma separated listed to the `--components` flag during deploy command. (ex. `zarf package deploy ./path/to/package.tar.zst --components=optional-component-1,optional-component-2`)


&nbsp;


## Composing Package Components
Existing components from other packages can be composed in new packages. This can be achieved by using the import field and providing a path to the zarf.yaml you wish to compose.

```yaml
components:
  - name: flux
    import:
     path: 'path/to/flux/package/directory/'
```

Unless you specify the component name in the import field, Zarf will try to import a component from the specified path that has the same name as the new component that is currently being defined. In the example above, since the new component is named 'flux' Zarf will import the 'flux' component from the specified path. If the new component is going to have a different name, you can specify the name of the package that needs to be imported in the import field.


```yaml
components:
  - name: flux
    import:
     path: 'path/to/flux/package/directory/'
     name: flux-v1.0.0
```

> Note: When importing a component, Zarf will copy all of the values from the original component expect for the `required` key. In addition, while Zarf will copy the values, you have the ability to override the value for the `description` key.

 Checkout the [composable-packages](https://github.com/defenseunicorns/zarf/blob/master/examples/composable-packages/zarf.yaml) example to see this in action.

&nbsp;

## What Makes Up A Component
Zarf components can contain different key/value pairs which you can learn more about here under the `components` section: [ZarfComponent Schema Docs](../3-zarf-schema.md#components)
