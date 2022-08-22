# Tiny Kafka

This example demonstrates using Zarf to deploy a simple operator example, in this case [Strimzi Kafka Operator](https://strimzi.io/).

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

&nbsp;

## Prerequisites

Before the magic can happen you have to do a few things:

1. Install [Docker](https://docs.docker.com/get-docker/). Other container engines will likely work as well but aren't actively tested by the Zarf team.

1. Install [KinD](https://github.com/kubernetes-sigs/kind). Other Kubernetes distros will work as well, but we'll be using KinD for this example since it is easy and tested frequently and thoroughly.

1. Clone the Zarf project &mdash; for the example configuration files.

1. Build the package using `zarf package create examples/tiny-kafka`

1. Create a Zarf cluster as described in the [Initializing a Cluster Walkthrough](../../docs/13-walkthroughs/1-initializing-a-k8s-cluster.md/)

&nbsp;

## Instructions

&nbsp;

### Deploy the package

Run the following command to deploy the created package to the cluster

```sh
zarf package deploy zarf-package-tiny-kafka-amd64.tar.zst --confirm
```

Wait a few seconds for the cluster to deploy the package.

&nbsp;

### Access Kafka

Testing requires JDK and the kafka tools: `sudo apt install openjdk-14-jdk-headless` (on Ubuntu). More details can be found at https://kafka.apache.org/quickstart. Steps to test:

1. Install JDK and extract the Kafka tools from the package `kafka.tgz`
2. Get the Nodeport: `NODEPORT=$(kubectl get service demo-kafka-external-bootstrap -n kafka-demo -o=jsonpath='{.spec.ports[0].nodePort}{"\n"}')`
3. For pub: `./bin/kafka-console-producer.sh --broker-list localhost:$NODEPORT --topic cool-topic`
4. For sub: `./bin/kafka-console-consumer.sh --bootstrap-server localhost:$NODEPORT --topic cool-topic`
