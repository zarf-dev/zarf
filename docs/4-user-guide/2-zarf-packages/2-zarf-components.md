---
sidebar_position: 2
---

# Understanding Zarf Components

The actual capabilities that Zarf Packages provided are defined within named components. These components define what dependencies they have and a declaritive definition of how it should be deployed. Each package can have as many components as the package creator wants but a package really isn't anything without at least one component.

Components can define a wide range of resources that it needs when the package it is a part of gets deployed. The schema for the components is listed below but a high level look at the things components can define include:
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
Zarf components can contain any of the following key/value pairs. Technically, none of the keys are required and you can use as many or few that makes sense to get to desired functionality:


<!-- TODO: @JPERRY this is out of date already.. Go through and redo..  -->
<!-- TODO: @JPERRY is the short mention of the 5-minute timeout for scripts enough? -->
```yaml


components:
  - name: <STRING> # A unique identifier for this component.
                   # The name can only contain alphabetical, numerical, or '-' characters.

    description: <STRING> # Message given to a user when deciding to enable this component or not

    required: <BOOLEAN> # If required, this component will always be deployed with the package

    secretName: <STRING> # The secret Zarf will use for the registry; default is 'zarf-registry'>
                         # The secret lives in the 'zarf' namespace.

    cosignKeyPath: <STRING> # Path to publickey to use for online resources signed by cosign.
                            # Signed files should be denoted with sget:// i.e. `sget://defenseunicorns/zarf-injector:0.4.3`

    images: <STRING LIST> # List of container images the component will use
                          # These images will be deployed to the Zarf provided docker registry

    repos: <STRING LIST> # List of git repos the component will use.
                         # These repos will be pushed into the gitea server.
                         # This also means the git-server component needs to be deployed during `zarf init`.
                         # Private repos need to have their credentialis listed in ~/.git-credentials


    files: <OBJ LIST>           # Files to move onto the system that will be doing the `zarf package deploy` command
      - source: <STRING>        # URL or path to where the file lives on the machine performing the `zarf package create` command
        shasum: <STRING>        # Optional value to verify remote sources
        target: <STRING>        # PAth to where the file will be placed on the system performing the `zarf package deploy` command
        executable: <BOOLEAN>   # Indicates whether or not executable permissions should be set on the file
        symlinks: <STRING LIST> # List of symlinks to create on the system performing the `zarf package deploy` command

    charts: <OBJ LIST>             # Helm charts to install during a package deploy
      - name: <STRING>             # Name of the component
      - url: <STRING>              # URL to where the chart is hosted (git or otherwise)
      - version: <STRING>          # Version of the chart to install
      - namespace: <STRING>        # Namespace to install the chart into
      - gitPath: <STRING>          # Path to the chart on the git repo
      - valuesFiles: <STRING LIST> # List of values files to use for the helm chart

    manifests: <OBJ LIST>             # Raw manifests that get converted into zarf-generated helm charts during deploy
      - name: <STRING>                # Name of the component
        namespace: <STRING>           # Namespace to install the manifest into
                                      # This defaults to 'default'
        files: <STRING LIST>          #
        kustomizations: <STRING LIST> #

    dataInjectors: <OBJ LIST> # data packages to push into a running k8s cluster
      - source: <STRING>      # TODO
        target: <OBJ>         # TODO
          namespace: <STRING> # TODO
          selector: <STRING>  # TODO
          path: <STRING>      # TODO

    import: <OBJ> # References a component in another Zarf package to import
                  # When 'import' is provided, the only other keys that matter are the 'name',
                  # 'required', 'description', and 'secretName' keys.
      path: <STRING> # Path to the zarf.yaml file of the component to import
      name: <STRING> # Optional name of the component to import
                     # If not provided, it defaults to the name of the component being defined

    scripts: <OBJ>       # custom commands that run before or after component deployment
  	  showOutput: <BOOLEAN> # Indicates if the output of the scripts should be sent through stdout/stderr
      timeoutSeconds: <INT> # Amount of time (in seconds) to wait for the script to complete before throwing an error
                              # The default time is 5 minutes
      retry: <BOOLEAN>      # Indicates if the script should be retried if it fails
      before: <STRING LIST> # List of scripts to run before the component is deployed
      after: <STRING LIST>  # List of scripts to run after the component is deployed
```