# Set Up And Install

:::caution Hard Hat Area
This page is still being developed. More content will be added soon!
:::


<!-- TODO: I @jperry am still confused about what the difference between this page the other install/setup sections should be.. -->
<!--       ex. The 'Getting Started' page has an 'Installing Zarf' section that I copied this from.. -->
# Installing Zarf
<!-- TODO: @JPERRY Look at how other tools/apps do their instillation instructions -->
Until we get Zarf into common package managers, you can install the Zarf CLI by:
1. Downloading the latest release version for your machine from our [GitHub release page](https://github.com/defenseunicorns/zarf/releases).
2. Move the downloaded file onto your path. This can be done in your terminal with the command `mv ~/Downloads/{DOWNLOADED_RELEASE_FILE} /usr/local/bin/zarf`
3. Test out the CLI within your terminal with the command `zarf -version`. The version you downloaded from GitHub should print to your terminal.

<br />
<br />

# Starting Up A Cluster

### Zarf Deployed K3s Cluster
<!-- TODO: Some duplicated information from the 'Common CLI Uses' page incoming... -->
Now that you have a Zarf CLI to work with, let's make a cluster that you can deploy to! The [init package](../user-guide/zarf-packages/the-zarf-init-package) comes with a built-in k3s cluster that you can deploy if you don't already have another cluster available. You can find the relevant [init package release](https://github.com/defenseunicorns/zarf/releases) on the GitHub releases page.

Once downloaded, you can install the init package by navigating to the directory containing the init package and running the command [zarf init](../user-guide/the-zarf-cli/cli-commands/zarf_init). Zarf will prompt you, asking if you want to deploy the k3s component, you can type `y` and hit enter to have Zarf startup a local single node k3s cluster on your current machine. Other useful information about initializing a cluster with Zarf is available in the [Initializing a Cluster](./set-up-and-install#initializing-) section later on in this page.

:::note
Depending on the permissions of your user, if you are installing k3s through the `zarf init` command you may need to run the command as a privileged user. This can be done by either:

1. Becoming a privileged user via the command `sudo su` and then running all the Zarf commands as you normally would.
2. Manually running all the Zarf commands as a privileged user via the command `sudo {ZARF_COMMAND_HERE}`.
3. Running the init command as a privileged user via `sudo zarf init` and then changing the permissions of the `~/.kube/config` file to be readable by the current user.
:::

### Any Other Cluster
<!-- TODO: Link to a support matrix of k8 distros -->
Zarf deploys to almost any cluster, you are not tied down to the K3s cluster that the [init package](../user-guide/zarf-packages/the-zarf-init-package) provides!  You could use local dockerized k8s cluster such as [k3d](https://k3d.io/v5.4.1/) or [KinD](https://kind.sigs.k8s.io/), Rancher's next-generation k8s distribution [RKE2](https://docs.rke2.io/), or cloud-provided clusters such as [eks](https://aws.amazon.com/eks/)


# Initializing a Cluster
<!-- TODO: Some duplicated information from the 'Common CLI Uses' page incoming... -->

Now that you have the CLI installed and a cluster to work with, let's get that cluster initialized so you can deploy application packages onto it! 

Initializing a cluster is necessary since almost all k8 clusters do not come with a container registry by default. This becomes an interesting 'chicken or the egg' problem since you don't have a container registry available to push your container registry image into. This problem is discussed more in the [init package](http://localhost:3000/docs/user-guide/zarf-packages/the-zarf-init-package#what-makes-the-init-package-special) page. 

During the initialization process, Zarf will create a 'zarf' namespace and deploy an in-cluster Docker registry (used to host container images future packages will need), a 'zarf agent' mutating webhook (to redirect outgoing requests into the internally hosted resources), an optional gitea server (to host the git repositories future packages will need), and a handful of secrets.

You can find the relevant [init package release](https://github.com/defenseunicorns/zarf/releases) on the GitHub releases page. Once downloaded, you can install the init package by navigating to the directory containing the init package and running the command [zarf init](../user-guide/the-zarf-cli/cli-commands/zarf_init). Zarf will prompt you, asking if you want to deploy the optional component, you can type `y` or `n` depending on your use case and needs.

Once the init command is finished, you can run `kubectl get pods -n zarf` to verify that the pods have come up healthy. You should expect to see two `agent-hook` pods,  a `zarf-docker-registry` pod, and optionally a `zarf-gitea` pod.


# Setup Complete!
<!-- TODO: FIX THIS LINK -->
At this point, you have the Zarf CLI installed and a k8s cluster running and initialized. You are now ready to start deploying packages to your cluster! The Walkthroughs section of the documentation will guide you through the process of deploying packages to your cluster, a good one to start with is the [Doom Walkthrough](https://google.com).