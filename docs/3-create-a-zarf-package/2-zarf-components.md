---
sidebar_position: 2
---

import Properties from '@site/src/components/SchemaItemProperties';
import FetchExampleYAML from '@site/src/components/FetchExampleYAML';

# Package Components

## Overview

The actual capabilities that Zarf Packages provided are defined within named components. These components define what dependencies they have and a declarative definition of how they should be deployed. Each package can have as many components as the package creator wants but a package isn't anything without at least one component. More documentation can be found [on the component actions docs page](../5-component-actions.md).

Components can define a wide range of resources that are needed when the package is deployed. The schema for components is located under the [`components` section of the package schema documentation](../3-zarf-schema.md#components). The below documentation showcases some of the different types of resources that can be defined in a component.

## Common Component Fields

There are certain fields that will be common across all component definitions. These fields are:

<Properties item="ZarfComponent" include={["name","description","default","required","group","cosignKeyPath","only"]} />

### Files

<Properties item="ZarfComponent" include={["files"]} />

<FetchExampleYAML example="import-everything" component="file-imports" branch="oci-package-compose" />

> explanation + example of local + relative + absolute + remote

### Helm Charts

<Properties item="ZarfComponent" include={["charts"]} />

<FetchExampleYAML example="import-everything" component="import-helm" branch="oci-package-compose" />

> explanation + example of local + relative + absolute + remote

### Kubernetes Manifests

<Properties item="ZarfComponent" include={["manifests"]} />

Raw Kubernetes manifests to deploy (by getting converted into zarf-generated helm charts and installed)

> explanation + example of local + relative + absolute + remote

### Container Images

<Properties item="ZarfComponent" include={["images"]} />

> explanation + example

### Git Repositories

<Properties item="ZarfComponent" include={["repos"]} />

* Git repositories to push into the git server the init-package created in the k8s cluster

> explanation + example

### Data Injections

<Properties item="ZarfComponent" include={["dataInjections"]} />

Data to push into a resource (i.e. a pod) in the k8s cluster

<FetchExampleYAML example="data-injection" component="with-init-container" />

### Zarf Components

Existing components from other packages can be composed in new packages. This can be achieved by using the import field and providing a path to the `zarf.yaml` you wish to compose.

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

> Note: When importing a component, Zarf will copy all of the values from the original component except for the `required` key. In addition, while Zarf will copy the values, you have the ability to override the value for the `description` key.

## Deploying Components

When deploying a Zarf package, the **components within a package are deployed in the order they are defined in the `zarf.yaml`**. The `zarf.yaml` configuration for each component also defines whether the component is 'required' or not. 'Required' components are always deployed without any additional user interaction while optional components are printed out in an interactive prompt asking the user if they wish to the deploy the component.

If you already know which components you want to deploy, you can do so without getting prompted by passing the components as a comma-separated list to the `--components` flag during the deploy command. (ex. `zarf package deploy ./path/to/package.tar.zst --components=optional-component-1,optional-component-2`)

Zarf components can contain different key/value pairs which you can learn more about here under the `components` section: [ZarfComponent Schema Docs](./5-zarf-schema.md#components)
