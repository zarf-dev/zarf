# Zarf Examples

The Zarf examples demonstrate different ways to utilize Zarf in your environment.  All of these examples follow the same general release pattern and assume an offline / air-gapped deployment target.

To build and deploy a demo, change directories to the example you want to try and run:

```shell
# This should be whatever example you want to try
cd <directory> # i.e. game

# This will create the zarf package
zarf package create

# This will prompt you to deploy the created zarf package
zarf package deploy
```

:::note

Examples are for demo purposes only and not meant for production use, they exist to demonstrate how to use Zarf in various ways. Modifying examples to fit production use is possible but will require additional configuration, time, and Kubernetes knowledge. Also, examples utilize software pulled from multiple sources and _some_ of them require authenticated access. Check the examples themselves for the specific accounts / logins required.

:::

&nbsp;

| Example                                                 |      Description      |
|---------------------------------------------------------|-----------------------|
| [component-choice](./component-choice/)                 |  Demo deploying packages with selectable components   |
| [component-scripts](./component-scripts/)               |  Demo building/deploying packages that contain custom scripts   |
| [composable-packages](./composable-packages/)           |  Demo building packages using components from other packages   |
| [data-injection](./data-injection/)                     |  Demo injecting data into a pod running on cluster  |
| [flux-test](./flux-test/)                               |  Demo deploying flux into a cluster to implement GitOps |
| [game](./game/)                                         |  Demo deploying old-school DOS games |
| [git-data](./git-data/)                                 |  Demo deploying git repos into a git server across the air gap   |
| [helm-alt-release-name](./helm-alt-release-name/)       |  Demo specifying an alternate relase name for a helm chart   |
| [package-variables](./package-variables/)               |  Demo specifying variables within a package   |
| [postgres-operator](./postgres-operator/)               |  Demo Postgres database deployment |
| [tiny-kafka](./tiny-kafka/)                             |  Demo Kafka cluster deployment  |
