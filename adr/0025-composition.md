# 25. Practical Component Composition

Date: 2024-07-02

## Status

Draft

## Context

Presently "composition" within Zarf is only possible at the package level. This can only be done with a
special kind of package, a "skeleton package". The actual "composition" of these "skeleton packages" into proper
packages is then supported by the `import` and `flavor` APIs.

We use "composition" (in quotations) here because this is not true composition. Specifically,
there is no way to declare a self-contained "optional" component that overrides Helm chart values
(or otherwise modifies the configuration of component(s) it is intended to be used with).

According to the
[Wikipeida article on _Composability_](https://en.wikipedia.org/wiki/Composability):

> A highly composable system provides components that can be selected and assembled in various combinations to satisfy
> specific user requirements. In information systems, the essential features that make a component composable are that it be:
>
> - self-contained (modular): it can be deployed independently – note that it may cooperate with other components,
>   but dependent components are replaceable
> - stateless: it treats each request as an independent transaction, unrelated to any previous request

Neither of these criteria are met in the context of Zarf components and packages. Here is a practical example from
[`defenseunicorns/uds-package-mattermost`](https://github.com/defenseunicorns/uds-package-mattermost/blob/5e02c2ceb7b0e097b7e6eb356b19eaff4c913613/zarf.yaml):

1. The `mattermost-(upstream|registry1)` component flavors depend on a `common` "skeleton package".
   The `common` package cannot be deployed independantly, which violates the "self-contained" principle.
2. The `mattermost-plugins` component is not "stateless". It must be declared first because it
   builds a container image during `onCreate` that is expected by the other components.
3. `mattermost-plugins` is not "self-contained" because, in order to use it,
   you must override Helm chart values declared by other components.

```yaml
kind: ZarfPackageConfig
metadata:
  name: mattermost
  description: "UDS Mattermost Package"
  version: "9.9.0-uds.0"

variables:
  - name: SUBDOMAIN
    description: "Subdomain for Mattermost"
    default: "chat"
  - name: DOMAIN
    default: "uds.dev"
  - name: ACCESS_KEY
    description: "Access Key for S3 compatible storage"
  - name: SECRET_KEY
    description: "Secret Key for S3 compatible storage"
  - name: DB_PASSWORD
    description: "Database Password for Mattermost"

components:
  - name: mattermost-plugins
    required: true
    images:
      - uds-package-mattermost/mattermost-extra-plugins:latest
    actions:
      onCreate:
        before:
          - dir: plugins
            cmd: |
              docker build . -t uds-package-mattermost/mattermost-extra-plugins:latest

  - name: mattermost
    required: true
    description: "Deploy Mattermost"
    import:
      path: common
    only:
      flavor: upstream
    charts:
      - name: mattermost-enterprise-edition
        valuesFiles:
          - values/upstream-values.yaml
    images:
      - appropriate/curl:latest
      - mattermost/mattermost-enterprise-edition:9.9.0

  - name: mattermost
    required: true
    description: "Deploy Mattermost"
    import:
      path: common
    only:
      flavor: registry1
      cluster:
        architecture: amd64
    charts:
      - name: mattermost-enterprise-edition
        valuesFiles:
          - values/registry1-values.yaml
    images:
      - registry1.dso.mil/ironbank/redhat/ubi/ubi9-minimal:9.4
      - registry1.dso.mil/ironbank/opensource/mattermost/mattermost:9.9.0
```

### Proposed Solutions

1. `components[].extends` (`string`): delcares this component as an extension (overlay) of another component.
   Similar to `flavor`, the resulting component is considered the "deployable unit" and cannot be deployed alongside the component it extends.
2. `components[].requires` (`[]string`): similar to `extends`, declares this component as an extension (overlay) of another component.
   However, unlike `extends`, the required component(s) are not replaced. Instead, the resulting component is considered an optional
   extention (overlay) to the required components. It can only be included when all required components are also included.
3. `images` (~~`[]string`~~ `[]{ name, newName?, newTag? }`): adopt
   [`ImageTagTransformer` semantics from Kustomize](https://kubectl.docs.kubernetes.io/references/kustomize/builtins/#_imagetagtransformer_)
   - `name`: the image name present in the component manifests
   - `newName` (optional): the new image name you wish to use
     (useful for changing registry locations)
   - `newTag` (optional): the new image tag you wish to reference
     (useful for updating version tags without modifying or relying on Helm chart values)

```yaml
kind: ZarfPackageConfig
metadata:
  name: mattermost
  description: "UDS Mattermost Package"
  version: "9.9.0-uds.0"

variables:
  - name: SUBDOMAIN
    description: "Subdomain for Mattermost"
    default: "chat"
  - name: DOMAIN
    default: "uds.dev"
  - name: ACCESS_KEY
    description: "Access Key for S3 compatible storage"
  - name: SECRET_KEY
    description: "Secret Key for S3 compatible storage"
  - name: DB_PASSWORD
    description: "Database Password for Mattermost"

components:
  - name: mattermost
    description: "Deploy Mattermost"
    required: true
    ## The `mattermost` component is now a self-contained, deployable unit.
    ## Thus, all the configuration from `common/zarf.yaml` has been inlined
    ## into this example. We no longer import a `common` "skeleton package".
    #
    # import:
    #   path: common
    # only:
    #   flavor: upstream
    charts:
      - name: uds-mattermost-config
        namespace: mattermost
        version: 0.1.0
        localPath: ./chart
        valuesFiles:
          - values/config-values.yaml
      - name: mattermost-enterprise-edition
        namespace: mattermost
        url: https://helm.mattermost.com
        gitPath: chart
        version: 2.6.55
        valuesFiles:
          - values/common-values.yaml
          - values/upstream-values.yaml
    ## Kustomize-style image replacements facilitate proper composition.
    ## This makes it easy for downstream components to override image tags
    ## without knowing anything about the Helm chart(s) being referenced
    ## nor their chart-specific supported `values`.
    #
    # images:
    #   - appropriate/curl:latest
    #   - mattermost/mattermost-enterprise-edition:9.9.0
    images:
      - name: appropriate/curl
        newTag: latest
      - name: mattermost/mattermost-enterprise-edition
        newTag: 9.9.0
    actions:
      onDeploy:
        after:
          - description: Validate Mattermost Package
            maxTotalSeconds: 300
            wait:
              cluster:
                kind: Packages
                name: mattermost
                namespace: mattermost
                condition: "'{.status.phase}'=Ready"

  - name: mattermost-registry1
    ## Previously a `required` component with the "registry1" `flavor`,
    ## `mattermost-registry1` is now defined simply as a component that directly
    ## extends the default `mattermost` component above.
    #
    # required: true
    # import:
    #   path: common
    # only:
    #   flavor: registry1
    #   cluster:
    #     architecture: amd64
    extends: mattermost
    required: false

    ## In this case (and as is the case with most `registry1` component flavors)
    ## the Helm chart values were only used to override image tags. We can now
    ## take advantage of a more robust and declarative `images` replacements API.
    #
    # charts:
    #   - name: mattermost-enterprise-edition
    #     valuesFiles:
    #       - values/registry1-values.yaml
    #       ## sample contents of `values/registry1-values.yaml`
    #       # mattermostApp:
    #       #   image:
    #       #     repository: registry1.dso.mil/ironbank/opensource/mattermost/mattermost
    #       #     tag: 9.9.0
    #       # initContainerImage:
    #       #   repository: registry1.dso.mil/ironbank/redhat/ubi/ubi9-minimal
    #       #   tag: 9.4
    # images:
    #   - registry1.dso.mil/ironbank/redhat/ubi/ubi9-minimal:9.4
    #   - registry1.dso.mil/ironbank/opensource/mattermost/mattermost:9.9.0
    images:
      - name: appropriate/curl
        newName: registry1.dso.mil/ironbank/redhat/ubi/ubi9-minimal
        newTag: 9.4
      - name: mattermost/mattermost-enterprise-edition
        newName: registry1.dso.mil/ironbank/opensource/mattermost/mattermost
        newTag: 9.9.0

  ## Finally, the most interesting example of *optionally* enabling injection of
  ## Mattermost plugins. This component is now "stateless" because it does nothing
  ## by default and it is "self-contained" because it both builds the required image
  ## and includes the necessary Helm chart values overrides
  ## (i.e. `mattermostApp.extraInitContainers`).
  - name: mattermost-plugins
    required: false
    ## Note that the semantics of `requires` is slightly different from `extends`.
    ## The idea is that `extends` signals intent to _replace_ the original component
    ## whereas `requires` signals that the originaly component(s) are being overlaid
    ## with additional configuration.
    requires: [ mattermost ]
    charts:
      - name: mattermost-enterprise-edition
        valuesFiles:
          - values/extra-plugins-values.yaml
          ## sample contents of `values/extra-plugins-values.yaml`:
          #
          # mattermostApp:
          #   extraInitContainers:
          #     - name: mattermost-extra-plugins
          #       image: uds-package-mattermost/mattermost-extra-plugins:latest
          #       imagePullPolicy: Always
          #       volumeMounts:
          #         - name: mattermost-plugins
          #           mountPath: /mattermost/plugins/
    images:
      - name: uds-package-mattermost/mattermost-extra-plugins
        newTag: latest
    actions:
      onCreate:
        before:
          - dir: plugins
            cmd: |
              docker buildx . -t uds-package-mattermost/mattermost-extra-plugins:latest
```

## Decision


## Consequences

