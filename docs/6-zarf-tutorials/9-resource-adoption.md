# Adopt Pre-Existing Resources

## Introduction

In this tutorial, you will create a test workload prior to initializing Zarf.  After that you will then use Zarf to adopt those workloads, so you can manage their future lifecycle with Zarf.

## System Requirements

- You'll need an internet connection to grab the Zarf Init Package if it's not already on your machine.

## Prerequisites

- Prior to this tutorial you'll want to have a working cluster.  But unlike our other tutorials you **don't want Zarf initialzed**.

- Zarf binary installed on your $PATH: ([Installing Zarf](../1-getting-started/index.md#installing-zarf))

## Youtube Tutorial
[![Tutorial: Adopt Pre-Existing Resources to Manage with Zarf](../.images/tutorials/adoption_thumbnail.png)](https://youtu.be/r3TBpMXtuNY "Adopt Pre-Existing Resources to Manage with Zarf")

## Creating a Test Component
We're going to use the manifests from the [Deploying a Retro Arcade](./3-deploy-a-retro-arcade.md) tutorial for this example.  So if you haven't yet, clone the Zarf repository, and navigate to the cloned repository's root directory. 

1. Create the dos-games namespace

<iframe src="/docs/tutorials/resource_adoption_namespace.html" width="100%" height="85px"></iframe>

2. Use the dos-games example manifests, to deploy the dos-games deployment and service to your Kubernetes cluster.

<iframe src="/docs/tutorials/resource_adoption_manifests.html" width="100%" height="110px"></iframe>

## Test to see that this is working

1. Use the `kubectl port-forward` command to confirm you've deployed the manifests properly.  

<iframe src="/docs/tutorials/resource_adoption_forward.html" width="100%" height="80px"></iframe>

2. Navigate to `http://localhost:8000` in your browser to view the dos-games application. It will look something like this:

![Connected to the Games](../.images/tutorials/games_connected.png)

:::note

Remember to press `ctrl+c` in your terminal when you're done with the port-forward.

:::

## Initialize Zarf

1. Use the [Initializing a K8s Cluster](./1-initializing-a-k8s-cluster.md) tutorial, to initialize Zarf in the cluster.

:::note

You'll notice the dos-games namespace has been excluded from Zarf management as it has the `zarf.dev/agent=ignore` label.  This means that Zarf will not manage any resources in this namespace.

<iframe src="/docs/tutorials/resource_adoption_ignored.html" width="100%" height="65px"></iframe>

:::

The iframe was pointing to the wrong file, and this likely would be better as an admonition.

## Create Zarf Package

1. Create the dos-games package with the `zarf package create` command.

<iframe src="/docs/tutorials/resource_adoption_package.html" width="100%" height="400px"></iframe>

## Deploy the Package, Adopting the Workloads

1. Use the `zarf package deploy` command with the `--adopt-existing-resources` flag to adopt the existing dos-games resources in the `dos-games` namespace.

<iframe src="/docs/tutorials/resource_adoption_deploy.html" width="100%" height="600px"></iframe>

## Test to see that this is working

1. You'll notice the dos-games namespace is no longer excluded from Zarf management as it has the `app.kubernetes.io/managed-by=zarf` label.  This means that Zarf will now manage any resources in this namespace.

<iframe src="/docs/tutorials/resource_adoption_adopted.html" width="100%" height="120px"></iframe>

2. You can also now use the `zarf connect` command to connect to the dos-games application. Again it will look something like this.
![Connected to the Games](../.images/tutorials/games_connected.png)

3. Again, remember to press `ctrl+c` in your terminal, when you're done with the connection.

<iframe src="/docs/tutorials/resource_adoption_connect.html" width="100%"></iframe>

## Conclusion

At this point the dos-game package is managed by Zarf and will behave just like a package initially deployed with Zarf.