# Deploy a Zarf Package

Zarf Packages are designed to be easily deployable on a variety of systems, including air-gapped systems. All of the necessary dependencies are included within the package, eliminating the need for outbound internet connectivity. When deploying the package onto a cluster, the dependencies contained in each component are automatically pushed into a Docker registry and/or Git server created by or known to Zarf on the air-gapped system.

Once the Zarf package has arrived in your target environment, run the `zarf package deploy` command to deploy the package onto your [Zarf initialized](../3-create-a-zarf-package/3-zarf-init-package.md) cluster. This command deploys the package's capabilities into the target environment, including all external resources required for the package. The `zarf.yaml` file included in the package will be used to orchestrate the deployment of the application according to the instructions provided.

:::tip

For a comprehensive tutorial of deploying a Zarf Package, see the [Deploying Zarf Packages tutorial](../5-zarf-tutorials/2-deploying-zarf-packages.md).

:::

## Deployment Options

Zarf provides a few options that can provide control over how a deployment of a Zarf Package proceeds in a given environment.  These are baked into a Zarf Package by a package creator and include:

- **Package Variables** - Templates resources with environment specific values such as domain names or secrets.
- **Optional Components** -  Allows for components to be optionally chosen when they are needed for a subset of environments.
- **Components Groups** - Provides a choice of one component from a defined set of components in the same component group.

## Additional Deployment-modes

Zarf normally expects to operate against a Kubernetes cluster that has been [Zarf initialized](../3-create-a-zarf-package/3-zarf-init-package.md), but there are additional modes that can be configured by package creators including:

- **YOLO Mode** - Yaml-OnLy Online mode allows for a faster deployment without requiring the `zarf init` command to be run beforehand. It can be useful for testing or for environments that manage their own registries and Git servers completely outside of Zarf.  Given this mode does not use the [Zarf Agent](../8-faq.md#what-is-the-zarf-agent) any resources specified will need to be manually modified for the environment.

- **Cluster-less** - Zarf normally interacts with clusters and kubernetes resources, but it is possible to have Zarf perform actions before a cluster exists (including [deploying the cluster itself](../5-zarf-tutorials/5-creating-a-k8s-cluster-with-zarf.md)).  These packages generally have more dependencies on the host or environment that they run within.

## Additional Resources

To learn more about deploying a Zarf package, you can check out the following resources:

- [Getting Started with Zarf](../1-getting-started/index.md): A step-by-step guide to installing Zarf and a description of the problems it seeks to solve.
- [Zarf CLI Documentation](../2-the-zarf-cli/index.md): A comprehensive guide to using the Zarf command-line interface.
- [The Package Deploy Lifecycle](./1-package-deploy-lifecycle.md): An overview of the lifecycle of `zarf package deploy`.
- [Deploying a Zarf Package Tutorial](../5-zarf-tutorials/3-deploy-a-retro-arcade.md): A tutorial covering how to deploy a package onto an initialized cluster.
- [The Zarf Init Package](../3-create-a-zarf-package/3-zarf-init-package.md): Learn about the 'init' package that is used to store resources for zarf packages.

## Typical Deployment Workflow:

The general flow of a Zarf package deployment on an existing initialized cluster is as follows:

```shell
# To deploy a package run the following:
$ zarf package deploy
# - Find and select the package using tab (shows packages from the local system)
# - Review Supply Chain and other pre-deploy information (clicking on the link to view SBOMs)
# - Type "y" to confirm package deployment or "N" to cancel
# - Enter any variables that have not yet been defined
# - Select any optional components that you want to add to the deployment
# - Select any component groups for this deployment

# Once the deployment finishes you can interact with the package
$ zarf connect [service name]
# - Your browser window should open to the service you selected
# - Not all packages define `zarf connect` services
# - You can list those that are available with `zarf connect list`
```

:::note

You can also specify a package locally, or via oci such as `zarf package deploy oci://ghcr.io/defenseunicorns/packages/dos-games:1.0.0-$(uname -m) --key=https://zarf.dev/cosign.pub`

:::
