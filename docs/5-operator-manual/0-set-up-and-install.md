# Set Up and Install

<!-- TODO: I @jperry am still confused about what the difference between this page the other install/setup sections should be.. -->
<!--       ex. The 'Getting Started' page has an 'Installing Zarf' section that I copied this from.. -->

## Installing Zarf

<!-- TODO: @JPERRY Look at how other tools/apps do their instillation instructions -->

In order to install the Zarf CLI, which is not yet available in common Linux package managers, you may follow the steps below:

1. Download the latest release version for your machine from our [GitHub release page](https://github.com/defenseunicorns/zarf/releases).
2. Transfer the downloaded file onto your path. This can be executed in your terminal with the command `mv ~/Downloads/{DOWNLOADED_RELEASE_FILE} /usr/local/bin/zarf`.
3. Verify the installation by testing the CLI within your terminal with the command `zarf -version`. If the installation is successful, the version of Zarf CLI that you have downloaded from GitHub should be displayed in your terminal.

:::note

For macOS or Linux, you may also install the Zarf CLI by using [brew](https://zarf.dev/install/).  

:::

## Starting up a Cluster

### Zarf Deployed K3s Cluster

<!-- TODO: Some duplicated information from the 'Common CLI Uses' page incoming... -->

Once the Zarf CLI is installed, you can create a cluster using the built-in K3s cluster available in the init package (if another cluster isn’t already available). You can find the relevant [init package release](https://github.com/defenseunicorns/zarf/releases) from the GitHub releases page.

Once downloaded, you can install the init package by navigating to the directory containing the package and execute the command [zarf init](../4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_init.md) to install it. Zarf will prompt you with a question, asking if you want to deploy the K3s component. Respond by typing "y" and pressing "Enter" to startup a local single node K3s cluster on your current machine. Additional information regarding initializing a cluster with Zarf is provided in the [Initializing a Cluster](./0-set-up-and-install.md#initializing-) section later on in this page.

:::note
Depending on the permissions of your user, if you are installing K3s through the `zarf init` command you may need to run the command as a privileged user. To accomplish this, you can:

1. Become a privileged user by running the command `sudo su` and then executing all the Zarf commands as usual.
2. Run the init command as a privileged user with the command `sudo zarf init`. Then, when Zarf is waiting for the cluster connection, copy `/root/.kube/config` to `~/.kube/config` and adjust the permissions of the `~/.kube/config` file to be readable by the current user.
:::

### Deploy to Your Preferred Cluster

<!-- TODO: Link to a support matrix of k8 distros -->

Zarf offers the flexibility of deploying to a wide range of clusters beyond the K3s cluster included in the [init package](../4-user-guide/2-zarf-packages/3-the-zarf-init-package.md). This means that you can utilize various options, including local dockerized K8s clusters such as [k3d](https://k3d.io/v5.4.1/) or [Kind](https://kind.sigs.k8s.io/), Rancher's next-generation K8s distribution [RKE2](https://docs.rke2.io/), or cloud-provided clusters such as [eks](https://aws.amazon.com/eks/). Such a diverse set of deployment choices frees you from being tethered to a single cluster option, allowing you to select the best-suited cluster environment for your specific needs.

## Initializing a Cluster

<!-- TODO: Some duplicated information from the 'Common CLI Uses' page incoming... -->

After installing the CLI and setting up a cluster, the next step is to initialize the cluster to enable the deployment of application packages.

Initializing a cluster is necessary since most K8 clusters do not come pre-installed with a container registry. This presents a challenging situation since pushing container images into a registry requires a registry to exist in the first place. For more information, please see the [init package](./../4-user-guide/2-zarf-packages/3-the-zarf-init-package.md) page.

As part of the initialization process, Zarf creates a dedicated namespace called `zarf` and deploys several essential components within the cluster. These include an in-cluster Docker registry (serves as the container image host for future packages), a `zarf agent` mutating webhook (to redirect outgoing requests to the internally hosted resources), and a set of secrets. Additionally, users can optionally deploy a gitea server that hosts the Git repositories needed for future packages. For more information regarding package components, see the [init package](./../4-user-guide/2-zarf-packages/3-the-zarf-init-package.md) page.

To access the relevant init package release, visit the [GitHub releases](https://github.com/defenseunicorns/zarf/releases) page. Once downloaded, navigate to the directory containing the init package and execute the command [`zarf init`](../4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_init.md) to install it. Zarf will prompt you to confirm whether you wish to deploy the optional component. You can type 'y' or 'n' depending on your specific use case.

After the initialization process is complete, you can verify that the pods have come up healthy by running the command `zarf tools kubectl get pods -n zarf`. You should expect to see two `agent-hook` pods, a `zarf-docker-registry` pod, and optionally a `zarf-gitea` pod.

## Set Up Complete

At this point, you have successfully installed the Zarf CLI and initialized a K8s cluster. You are now ready to begin deploying packages to your cluster. The [Walkthroughs](../13-walkthroughs/index.md) section of the documentation provides a step-by-step guide on how to deploy packages to your cluster, and the [Doom Walkthrough](../13-walkthroughs/2-deploying-doom.md) is a great place to start. Follow along with the guide to learn more about deploying packages to your cluster and get started with your first deployment.
