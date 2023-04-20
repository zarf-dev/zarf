# Zarf Deployment Web UI

Zarf has a Deoployment Web UI built in. The Deployment Web Ui supports a nubmer of Zarf features used during the package deployment process. For users who prefer not to use the command line tool, the Web UI creates a simple experience to deploy and manage Zarf clusters and packages. The Web UI can be used to connect to connect to existing clusters (via Kubeconfig), initalize a cluster, deploy packages into a cluster, update packages in the cluster, and remove packages from the cluster. 

The Zarf Web UI mirros the functionality of the Zarf CLI commands, but with a more intuitive flow and familuar web application patterns for non-technical users. The web UI does not offer any additional commands or core functionaliity to Zarf. 

## Open the Zarf Deployment Web UI

The Zarf Deployment Web UI can be easily spun up with a single command from the CLI terminal. 

Follow these steps to get started using the Web UI

1. Step one: Install Zarf binary
2. Step two: Open CLI terminal
3. Step three: Type in the following command: ```zarf dev UI```

![GIF showing the Web UI lauch from the CLI terminal](../.images/dashboard/Web_UI__Launch_w__Cluster_AdobeExpress.gif)

## USing Zarf Deployment Web UI

### Cluster Connection Status

When Zarf is running it automatically searches for a Kubeconfig on the local machine. If the Kubeconfig is found, it searches the defualt cluster to determine if it is a Zarf cluster (Initialized). There are two different cluster statuses the Web UI will display based on the state of the cluster found. 

#### Cluster not Connected (Not Initizalized)

![Web UI shows organge warning status and message "cluster not connected" on the cluster card](../.images/dashboard/Web%20UI%20-%20Cluster%20Not%20Connected.png)

1. Shown when there is no Kubeconfig found on the machine.
2. Shown when there is a Kubeconfig foudn on the machine but it has not currently connected to Zarf.

#### Cluster Connected (Initialized)

If Zarf finds a cluster in the Kubeconfig that has Zarf resources on it it will automatically connect to the cluster and display the cluster details on the Web UI.

![Web UI shows cluster metat data in on the cluster card when a connected cluster is found](../.images/dashboard/Web%20UI%20-%20Status%20Cluster%20connected.png)

3. Shown when there is a Kubeconfig found on the machine with a default cluster that has Zarf resources in it.


### Connect to Existing Cluster 

The Zarf Web UI makes connecting to existing clusters easy. From the packages users will be able to tell if a cluster is connected by the st

To connect to an existing cluster:

1. Have a cluster running on your machine
2. Click connect cluster
3. 

### Deploy K8 and Cluster with init package with Cluster
### Deploy additional packages
### Remove Package
### Update Package

## Technical Details

The web UI is packaged into the Zarf binay, so you don't have to worry about additional dependencies or trying to install it yourself! The Web UI is served through your machines local browser, running as a local host, and utlizes as the backend. 

Use the Zarf Deployment UI to execute the existing Zarf CLI commands. 
- [Zarf tools Kubectl](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_tools_kubectl.md)
- [Zarf tools Kubectl top](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/)
- [Zarf Init](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_init.md)
- [Zarf Package Deploy](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/)
- [Zarf Package Remove](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_package_remove.md)
- [Zarf Pacakge List](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_package_list.md)
- [Zarf Package Inspect](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_package_inspect.md) (coming soon)
- [Zarf Tools Sbom](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_tools_sbom.md) (Coming soon)
- [Zarf Connect](/docs/4-user-guide/1-the-zarf-cli/100-cli-commands/zarf_connect.md) (coming soon) 

:::info

All other zarf [CLI commands](../4-user-guide/1-the-zarf-cli/100-cli-commands/) will require interfacing with the CLI directly.

::: 