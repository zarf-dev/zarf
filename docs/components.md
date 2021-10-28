# Zarf Components

While Zarf is fairly unopinionated regarding what runs in your cluster, that is not to say that it's completely indifferent. It has _definite_ opinions, for example, about how to meet many common production application functionality needs&mdash;we call these opinions **components**.

Think of components as something like named capabilities.

They're intended to fill in the space _around_ your apps; to do things that must be done but which aren't your core concern&mdash;things like running application logging & monitoring services, or installing pre-configured cluster management tooling.

Backed by tooling you already know (and love) & structured to fill the gaps you don't want to have to think about, Zarf's components tie together common software sets and give you an easy, _named_ way to get them into your clusters.

&nbsp;

## Available components

This is the list of components that Zarf currently supports along with the "magic strings" you can pass through `zarf init --components` in order to use them:

|--components       |Description|
|---                |---|
|k3s                |Installs a lightweight Kubernetes Cluster on the local host&mdash;[k3s](https://k3s.io/)&mdash;and configures it to startup on boot.|
|management         |Installs tools for _managing_ the Zarf cluster from the local host, including: [k9s](https://k9scli.io/)|
|container-registry |Adds a container registry service&mdash;[docker registry](https://docs.docker.com/registry/)&mdash;into the cluster.|
|logging            |Adds a log monitoring stack&mdash;[promtail / loki / graphana (a.k.a. PLG)](https://github.com/grafana/loki)&mdash;into the cluster.|
|gitops-service     |Adds a [GitOps](https://www.cloudbees.com/gitops/what-is-gitops)-compatible source control service&mdash;[Gitea](https://gitea.io/en-us/)&mdash;into the cluster.|

&nbsp;

## Further reading

For more detail&mdash;like which components are required vs. those which are merely on by default&mdash;there's no better place to check than the source: [zarf.yaml](../zarf.yaml).