# Zarf Examples

The Zarf examples demonstrate different ways to utilize Zarf in your environment.  All of these examples follow the same general release pattern and assume an offline / air-gapped deployment target.

To build and deploy a demo, change directories to the example you want to try and run:

```shell
# This should be whatever example you want to try (game example in this case)
cd game

# This will create the zarf package
zarf package create

# This will prompt you to deploy the new zarf package
zarf package deploy
```

> **NOTE**: Examples are for demo purposes only and not meant for production use, they exist to demonstrate how to use Zarf in various ways. Modifying examples to fit production use is possible but will require additional configuration, time, and Kubernetes knowledge. Also, examples utilize software pulled from multiple sources and _some_ of them require authenticated access. Check the examples themselves for the specific accounts / logins required.


&nbsp;


| Example                                                          |      Description      |
|------------------------------------------------------------------|-------------|
| [component-](./component-/README.md)           |  Demo building packages using components from other packages   |
| [composable-packages](./composable-packages/README.md)           |  Demo building packages using components from other packages   |
| [data-injection](./data-injection/README.md)                     |  Demo injecting data into a pod running on cluster  |
| [game](./game/README.md)                                         |  Demo deploying old-school DOS games |
| [gitops-data](./gitops-data/README.md)                           |  Demo deploying git repos into a git-server running in cluster   |
| [postgres-opereator](./postgres-operator/README.md)              |  Demo Postgres database deployment |
| [tiny-kafka](./tiny-kafka/README.md)                             |  Demo Kafka cluster  |
