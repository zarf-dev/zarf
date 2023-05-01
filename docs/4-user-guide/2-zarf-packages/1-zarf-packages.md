---
sidebar_position: 1
---

# Understanding Zarf Packages

Zarf offers a comprehensive solution for deploying system software or capabilities while fully disconnected. This is accomplished through the use of a single tarball archive, which includes all necessary components and is defined by a `zarf.yaml` file.

Zarf Packages are created while you are connected to internet or intranet resources defined in a Zarf package configuration.  This allows these resources to be pulled and stored within a package archive and be deployed on disconnected systems without requiring a connection to the outside world.

The `zarf.yaml` file also defines declarative instructions for the deployment of package capabilities, which are automatically executed on package deployment and are represented in code. This ensures reproducibility across different systems without the need for manual configuration.

Zarf Packages consist of functional blocks, known as components. Components can be optional, providing greater flexibility in package usage. Further details on components can be found on the [Zarf Components](./2-zarf-components.md) page.

## Deploying onto Air-gapped Systems

Zarf Packages are built to include all necessary dependencies within the package itself, making it particularly useful for deploying to air-gapped systems. This eliminates the need for outbound internet connectivity, making the packages easily distributable and executable on a variety of systems, including edge, embedded systems, secure cloud, data centers, or local environments.

When deploying a package onto a cluster, the dependencies contained in each component are automatically pushed into a Docker registry and/or Git server created by or known to Zarf on the air-gapped system. This enables any later steps to utilize these dependencies as needed.

## Types of Zarf Packages

There are two types of Zarf Packages, the `ZarfInitConfig` and the `ZarfPackageConfig`, which are distinguished by the `kind:` field and specified in the `zarf.yaml` file.

Throughout the rest of the documentation, we will refer to the `ZarfInitConfig` as an `init config` package or `init` package, and to the `ZarfPackageConfig` as simply a "package".

### ZarfInitConfig

The init package is used to initialize a cluster, making it ready for deployment of other Zarf Packages. It must be executed once on each cluster that you want to deploy another package onto, even if multiple clusters share the same host. For additional information on the init package, we provide detailed documentation on the Zarf ['init' package page](./3-the-zarf-init-package.md).

If there is no running cluster, the init package can be used to create one. It has a deployable K3s cluster component that can be optionally deployed on your machine. Usually, an init package is the first Zarf Package to be deployed on a cluster as other packages often depend on the services installed or configured by the init package. If you want to install a K8s cluster with Zarf, but you don't want to use K3s as your cluster, you will need to create or find another Zarf Package that will stand up your cluster before you run the zarf init command.

:::note

To clarify, in most cases, the first Zarf Package you deploy onto a cluster should be the init package since other packages often depend on the services it installs onto your cluster. However, if you don't want to use the K3s distribution included in the init package or if you already have a preferred K8s distribution, you can deploy the distribution package first, followed by the init package, and then any other packages you want. This only applies if you don't have a K8s cluster yet.

:::

During the initialization process, Zarf will seed your cluster with a container registry to store images that other packages may require. Additionally, the init package has the option to deploy other features to your cluster, such as a Git server to manage your repositories or a PLG logging stack that allows you to monitor the applications running on your cluster.

#### Using the init-package

To initialize your cluster, you need to run the command `zarf init`. This command will search for a file with the specific naming convention: `zarf-init-{ARCHITECTURE}-{VERSION}.tar.zst`. The architecture must match that of the cluster you are deploying to. If you are deploying to a cluster with a different architecture, you will need to specify the name of the architecture you are deploying on with the `-a` flag. For example, if you are on an arm64 machine but are deploying on an amd64 machine, you will run `zarf init -a amd64`.

Init packages can also be run with `zarf package deploy zarf-init-{ARCHITECTURE}-{VERSION}.tar.zst`.

You do not need to create init configs by yourself unless you want to customize how your cluster is installed/configured. For example, if you want to use the init process to install a specifically configured K3s cluster onto your host machine, you can create a specific package to do that before running the init package.

### ZarfPackageConfig

`ZarfPackageConfig` refers to any package that is not an init package and is used to define specific capabilities that you want to deploy onto your initialized cluster.

To deploy a Zarf Package, you can use the command `zarf package deploy`. This will prompt you to select from all of the files in your current directory that match the name `zarf-package-*.tar.zst`. Alternatively, if you already know which package you want to deploy, you can simply use the command `zarf package deploy {PACKAGE_NAME}`.

During the deployment process, Zarf will leverage the infrastructure created during the 'init' process (such as the Docker registry and Git server) to push all the necessary images and repositories required for the package to operate.

## Package Components

Zarf Packages consist of smaller units known as 'components'. These components are further defined on the [Zarf Components page](./2-zarf-components.md), but to summarize, they represent the named capabilities that packages offer. For additional information regarding the `zarf.yaml` package structure, please refer to the [Zarf Package Schema](../3-zarf-schema.md) documentation.

## Creating a Zarf Package

The following list outlines the dependencies for creating a Zarf Package:

- A local K8s cluster to work with ([K3s](https://k3s.io/)/[k3d](https://k3d.io/v5.4.1/)/[Kind](https://kind.sigs.k8s.io/docs/user/quick-start#installation)).
- A Zarf CLI ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../../2-the-zarf-cli/0-building-your-own-cli.md)).
- A Zarf init package ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../../2-the-zarf-cli/0-building-your-own-cli.md)).

The process of defining a package is elaborated in detail on the [Creating a Package](../../13-walkthroughs/0-using-zarf-package-create.md) page. Once a package has been defined, building it is a relatively straightforward task.

The `zarf package create` command locates the `zarf.yaml` file in the current directory and constructs the package from that file. The command utilizes internet or intranet resources to retrieve all the required assets and stores them in a temporary directory. After the required resources have been obtained, Zarf generates a tarball of the temporary directory and performs necessary cleanup actions.


## Inspecting a Created Package

To inspect the contents of a Zarf Package, you can use the command `zarf package inspect` followed by the path to the package file. This will print out the contents of the `zarf.yaml` file that defines the package. For example, if your package is located at `./path/to/package.tar.zst`, you can run `zarf package inspect ./path/to/package.tar.zst` to view the contents of the `zarf.yaml` file.
