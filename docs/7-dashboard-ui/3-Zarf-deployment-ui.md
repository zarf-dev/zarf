# Zarf Deployment Web UI

Zarf has a Deployment Web UI built in that supports a number of Zarf features used during the package deployment process. For users who prefer not to use the command line tool, the Web UI creates a simple experience to deploy and manage Zarf clusters and packages. The Web UI can be used to connect to existing clusters (via a Kubeconfig), initialize a cluster, deploy packages into a cluster, update packages in the cluster, and remove packages from the cluster. 

The Zarf Web UI mirrors the functionality of the Zarf CLI commands, but with a more intuitive flow and familiar web application patterns for non-technical users. The web UI does not offer any additional commands or core functionality to Zarf. 

## Open the Zarf Deployment Web UI

The Zarf Deployment Web UI can easily be spun up with a single command from the CLI. 

Follow these steps to get started using the Web UI

1. Step one: Install the Zarf binary
2. Step two: Open a terminal shell
3. Step three: Type in the following command: ```zarf dev UI```

![GIF showing the Web UI lauch from the CLI terminal](../.images/dashboard/Web_UI__Launch_w__Cluster_AdobeExpress.gif)

## Using the Zarf Deployment Web UI

### Cluster Connection Status

When Zarf is running it automatically searches for a Kubeconfig on the local machine. If the Kubeconfig is found, it searches the default cluster to determine if it is a Zarf cluster (i.e. initialized). There are two different cluster statuses the Web UI will display based on the state of the cluster found. 

#### Cluster not Connected (Not Initizalized)

![Web UI shows organge warning status and message "cluster not connected" on the cluster card](../.images/dashboard/Web%20UI%20-%20Cluster%20Not%20Connected.png)

1. Shown when there is no Kubeconfig found on the machine.
2. Shown when a Kubeconfig is found on the machine, but Zarf has not been deployed and set up in the cluster. 

#### Cluster Connected (Initialized)

If Zarf finds a cluster in the Kubeconfig that has Zarf resources on it it will automatically connect to the cluster and display the cluster details on the Web UI.

![Web UI shows cluster metat data in on the cluster card when a connected cluster is found](../.images/dashboard/Web%20UI%20-%20Status%20Cluster%20connected.png)

1. Shown when there is a Kubeconfig found on the machine with a default cluster that has Zarf resources in it.


### Connect to Existing Cluster 

The Zarf Web UI makes connecting to existing clusters easy. When on the packages page, if there is no Zarf cluster currently connected, select the connect cluster button. If Zarf finds a Kubeconfig it will ask the user if they want to connect to the the default cluster context. 

:::Tip 

Zarf can only read the default cluster in your Kubeconfig file, if you wish to connect to a different cluster in the Kubeconfig you will need to change it to the default cluster in the terminal. See the Kubernetes documentation on [how to configure access to multiple clusters](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/).

::: 

Follow these steps to connect to an existing cluster:

1. Be sure to have a Kubeconfig on your machine with the cluster you wish to connect to set as the default cluster.
2. Click connect cluster button on cluster card
3. Select cluster name in the dialog and click connect
4. Select a ZarfInitConfig package to deploy into cluster
5. Follow Package deployment prompts to deploy package and initialize cluster as a Zarf cluster.

### Deploy K8 and Cluster with init package with Cluster

If you do not have access to an existing cluster, or would simply like to spin up a new cluster. You can do that by deploying a ZarfInitConfig package and selecting the optional K3s component. 

:::info

This option is currently only available for Linux machines with access to the `root` user.

:::

Follow these steps to deploy and initialze a new cluster:

1. Click deploy package button (on the Deployed packages table)
2. Select a ZarfInitPackage from the list
3. Toggle the switch on for the K3s component to select it for deployment
4. Complete package deployment steps to spin up and initialze your new Zarf cluster.

### Deploy additional packages

Once you have a cluster connected to Zarf, you can deploy additional packages into the cluster. 

Steps to deploy additional packages into cluster:

1. Click deploy package button on the Deployed packages table
2. Select the package you wish to deploy from the list
3. Complete the package deployment steps 

### Additional Package Commands

Once a package is deployed into the cluster, the Web UI offers additional commands that can be executed for a package. To view these commands click on the vertical ellipsis at the end of the table row for the package you wish to act upon. The Web UI currently supports the following package commands:

- Update: Use when you wish to update a package with a new version of the same package.
- Remove: Use when you wish to remove a package and all of it's resources from the cluster. This cannot be undone.

![Web UI deployed packages table with a context menu showing additional package commands](../.images/dashboard/Web%20UI%20-%20package%20commands.png


## Technical Details

The web UI is packaged into the Zarf binary, so you don't have to worry about additional dependencies or trying to install it yourself! The Web UI is served through your machine's local browser, running on `localhost`, and utilizes the Zarf go binary as the backend. 

Use the Zarf Deployment UI to execute the existing Zarf CLI commands:
- [Zarf tools Kubectl top](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/)
- [Zarf Init](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_init.md)
- [Zarf Package Deploy](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/)
- [Zarf Package Remove](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_package_remove.md)
- [Zarf Package List](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_package_list.md)
- [Zarf Package Inspect](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_package_inspect.md) (coming soon)
- [Zarf Tools Sbom](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_tools_sbom.md) (Coming soon)
- [Zarf Connect](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_connect.md) (coming soon) 

:::info

All other zarf [CLI commands](../4-user-guide/1-the-zarf-cli/100-cli-commands/) will require interfacing with the CLI directly.

::: 
