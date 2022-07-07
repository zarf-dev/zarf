# Walkthroughs

This section of the documentation has a collection of walkthroughs that will help you get more familiar with Zarf and its features. The walkthroughs assume that you have a very basic understanding of what Zarf is and aims to help expand your working knowledge of how to use Zarf and what Zarf is capable of doing.



## Walk Through Prerequisites
<!-- TODO: Should we add `kubectl` as a pre req? -->
If a walkthrough has any prerequisites, it will be listed at the beginning of the walkthrough with instructions on how to fulfill them.
Almost all walkthroughs will have the follow prerequisites/assumptions:
1. The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([git clone instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
1. You have a Zarf binary installed on your $PATH: ([Zarf Install Instructions](../getting-started#installing-zarf))
1. You have an init-package built/downloaded: ([init-package Build Instructions](./creating-a-zarf-package)) or ([Download Location](https://github.com/defenseunicorns/zarf/releases))
1. Have a kubernetes cluster running/available (ex. [k3s](https://k3s.io/)/[k3d](https://k3d.io/v5.4.1/)/[KinD](https://kind.sigs.k8s.io/docs/user/quick-start#installation))
1. You have kubectl installed: ([kubectl Install Instructions](https://kubernetes.io/docs/tasks/tools/#kubectl))

<br />

## Setting Up a Local Kubernetes Cluster
While Zarf is able to deploy a local k3s Kubernetes cluster for you, (as you'll find out more in the [Creating a K8s Cluster with Zarf](./creating-a-k8s-cluster-with-zarf) walkthrough), that k3s cluster will only work if you are on a root user on a Linux machine. If you are on a Mac, or you're on Linux but don't have root access, you'll need to setup a local dockerized Kubernetes cluster manually. We provide instructions on how to quickly set up a local k3d cluster that you can use for the majority of the walkthroughs.


### Install k3d
1. Install Docker: [Docker Install Instructions](https://docs.docker.com/get-docker/)
2. Install k3d: [k3d Install Instructions](https://k3d.io/#installation)


### Start up k3d cluster

```bash
k3d cluster create      # Creates a k3d cluster
                        # This will take a couple of minutes to complete


kubectl get pods -A    # Check to see if the cluster is ready 
```

### Tear Down k3d CLuster 

```bash
k3d cluster delete      # Deletes the k3d cluster
```
