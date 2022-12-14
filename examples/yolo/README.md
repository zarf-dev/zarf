# YOLO Mode
This example demonstrates YOLO mode, an optional mode for using Zarf in a fully connected environment where users can bring their own external container registry and Git server.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::


## Prerequisites
- A running K8s cluster. Note that the cluster does not need to have the Zarf init package installed or any other Zarf-related bootstrapping.

## Instructions
Create the package:
```sh
zarf package create
```

### Deploy the package
Run the following command to deploy the created package to the cluster

```sh
zarf package deploy zarf-package-yolo-arm64.tar.zst --confirm
```

Wait a few seconds for the cluster to deploy the package, you should see the following output when the package has been successfully deployed.

```
 Connect Command    | Description
 zarf connect doom  | Play doom!!!
 zarf connect games | Play some old dos games 🦄
```
Run the specified `zarf connect <game>` command to connect to the deployed workload (ie. kill some demons). Note that the typical Zarf registry, Gitea server and Zarf agent pods are not present in the cluster. This means that the game's container image was pulled directly from the public registry and the URL was not mutated by Zarf.
