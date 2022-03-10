# Zarf Components

While Zarf is fairly unopinionated regarding what runs in your cluster, that is not to say that it's completely indifferent. It has _distinct_ opinions, for example, about how to meet many common production application functionality needs&mdash;we call these opinions **components**.

Think of components as something like named capabilities.

They're intended to fill in the space _around_ your apps; to do things that must be done but which aren't your core concern&mdash;things like running application logging & monitoring services, or installing pre-configured cluster management software.

Backed by tooling you already know (and love) & structured to fill the gaps you don't want to have to worry over, Zarf's components tie together common software sets and give you an easy, _named_ way to get them into your clusters.

&nbsp;


## Mandatory components

Zarf's work necessitates that some components are "always on" (a.k.a. required & cannot be disabled). Those include:

|                   |Description|
|---                |---|
|container-seed-registry|Adds a container registry so Zarf can bootstrap itself into the cluster.|
|container-registry |Adds a container registry service&mdash;[docker registry](https://docs.docker.com/registry/)&mdash;into the cluster.|

&nbsp;


## Additional components

In addition to those that are always installed, Zarf's optional components provide additional functionality and can be enabled as & when you need them.

These optional components are listed below along with the "magic strings" you pass to `zarf init --components` to pull them in:

|--components       |Description|
|---                |---|
|k3s                |Installs a lightweight Kubernetes Cluster on the local host&mdash;[k3s](https://k3s.io/)&mdash;and configures it to start up on boot.|
|logging            |Adds a log monitoring stack&mdash;[promtail / loki / graphana (a.k.a. PLG)](https://github.com/grafana/loki)&mdash;into the cluster.|
|gitops-service     |Adds a [GitOps](https://www.cloudbees.com/gitops/what-is-gitops)-compatible source control service&mdash;[Gitea](https://gitea.io/en-us/)&mdash;into the cluster.|

&nbsp;

## Composing Package Components
Existing components and packages within a zarf.yaml can be composed in new packages. This can be achieved by using the import field and providing a path the zarf.yaml you wish to compose. Checkout the  [composable-packages](../examples/composable-packages/zarf.yaml) example.
```yaml
components:
  - name: flux
    import: 
     path: 'path/to/flux/package/directory/'
```

&nbsp;

## Further reading

For more detail&mdash;like which components are on/off by default&mdash;there's no better place to check than the source: [zarf.yaml](../zarf.yaml).
