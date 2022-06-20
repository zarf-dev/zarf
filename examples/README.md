# Zarf Examples

The Zarf examples demonstrate different ways to utilize Zarf in your environment.  All of these examples follow the same general release pattern and assume an offline / air-gapped deployment target.

To test create a virtual area to test all examples, you can run `make all` or `make vm-init` if you've already run the examples before. Run `make vm-destroy` to clean up.

> **NOTE**: Examples are for demo purposes only and not meant for production use, they exist to demonstrate how to use Zarf in various ways. Modifying examples to fit production use is possible but will require additional configuration, time, and Kubernetes knowledge. Also, examples utilize software pulled from multiple sources and _some_ of them require authenticated access. Check the examples themselves for the specific accounts / logins required.


&nbsp;


| Example                                                          |      Description      |
|------------------------------------------------------------------|-------------|
| [big-bang](../packages/big-bang-core/README.md)                        |  Demo BigBang v1.33.0 with all of its core services |
| [composable-packages](./composable-packages/README.md)           |  Demo building packages using components from other packages   |
| [data-injection](./data-injection/README.md)                     |  Demo injecting data into a pod running on cluster  |
| [game](./game/README.md)                                         |  Demo deploying old-school DOS games |
| [gitops-data](./gitops-data/README.md)                           |  Demo deploying git repos into a git-server running in cluster   |
| [istio-with-separate-cert](./istio-with-separate-cert/README.md) |  Demo deployment of Istio with a separate TLS cert |
| [postgres-opereator](./postgres-operator/README.md)              |  Demo Postgres database deployment |
| [single-big-bang-package](./single-big-bang-package/README.md)   |  Demo deployment of a single BigBang service (Twistlock)   |
| [tiny-kafka](./tiny-kafka/README.md)                             |  Demo Kafka cluster  |
