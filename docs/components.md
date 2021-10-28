# Zarf Components

Zarf is pretty unopinionated regarding what runs in your cluster but it _does_ have an opinion about how to help you fill in some common functionality gaps. We call these opinions **components**.

Think of components as something like named capabilities.

They're intended to fill in the space _around_ your apps; to do things that must be done but which aren't your core concern&mdash;things like running application logging & monitoring services, or installing (pre-configured) cluster management tooling.



&nbsp;


## Available components

|--components       |Description|
|---                |---|
|k3s                |Installs a lightweight Kubernetes Cluster on the local host&mdash;[k3s](https://k3s.io/)&mdash;and configures it to startup on boot.|
|management         |Installs tools for _managing_ the Zarf cluster from the local host, including: [k9s](https://k9scli.io/)|
|container-registry |Adds a container registry service&mdash;[docker registry](https://docs.docker.com/registry/)&mdash;into the cluster.|
|logging            |Adds a log monitoring stack&mdash;[promtail / loki / graphana (a.k.a. PLG)](https://github.com/grafana/loki)&mdash;into the cluster.|
|gitops-service     |Adds a [GitOps](https://www.cloudbees.com/gitops/what-is-gitops)-compatible source control service&mdash;[Gitea](https://gitea.io/en-us/)&mdash;into the cluster.|

&nbsp;

## Further reading

For more detail&mdash;like which components are required vs. those which are merely on by default&mdash;there's no better place to check that the source!

Check it out in the project root: [zarf.yaml](../zarf.yaml).