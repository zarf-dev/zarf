# Create a Zarf Package

Zarf enables you to consolidate portions of the internet into a single package that can be conveniently installed at a later time. A Zarf Package is a single tarball file that includes all of the resources and instructions required for efficiently managing a system or capability, even when entirely disconnected from the internet. In this context, a disconnected system refers to a system that either consistently operates in an offline mode or occasionally disconnects from the network.

Once defined, a Zarf Package contains comprehensive instructions on assembling various software components that are to be [deployed onto the targeted system](../4-deploy-a-zarf-package/index.md). The instructions are fully "declarative", meaning that all components are represented by code and automated, eliminating the need for manual intervention.

## Additional Resources

To learn more about creating a Zarf package, you can check out the following resources:

- [Getting Started with Zarf](../1-getting-started/index.md): A step-by-step guide to installing Zarf and a description of the problems it seeks to solve.
- [Zarf CLI Documentation](../2-the-zarf-cli/index.md): A comprehensive guide to using the Zarf command-line interface.
- [Understanding Zarf Packages](./1-zarf-packages.md): A breakdown of the kinds of Zarf packages, their uses and how they work.
- [Understanding Zarf Components](./2-zarf-components.md): A breakdown of the primary structure that makes up a Zarf Package.
- [Zarf Schema Documentation](./4-zarf-schema.md): Documentation that covers the configuration available in a Zarf Package definition.
- [The Package Create Lifecycle](./5-package-create-lifecycle.md): An overview of the lifecycle of `zarf package create`.
- [Creating a Zarf Package Tutorial](../5-zarf-tutorials/0-creating-a-zarf-package.md): A tutorial covering how to take an application and create a package for it.

## Typical Creation Workflow:

The general flow of a Zarf package deployment on an existing initialized cluster is as follows:

```shell
# To create a package run the following:
$ zarf package create <directory>
# - Enter any package templates that have not yet been defined
# - Type "y" to confirm package creation or "N" to cancel

# Once the creation finishes you can interact with the built package
$ zarf inspect <package-name>.tar.zst
# - You should see the specified package's zarf.yaml
# - You can also see the sbom information with `zarf inspect <package-name>.tar.zst --sbom`
```
