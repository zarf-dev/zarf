---
sidebar_position: 1
---

# Understanding Zarf Packages

A Zarf package is a single binary Tarball that contains everything you need to deploy a system or capability while fully disconnected. Zarf packages are defined by a `zarf.yaml` file.

Zarf packages are built while 'online' and connected to whatever is hosting the dependencies your package definition defined. When being built, all these defined dependencies are pulled from the internet and stored within the tarball package. Because all the dependencies are now within the tarball, the package can be deployed on to disconnected systems that don't have connection to the outside world.

The `zarf.yaml` file, that the package builds from, defines declarative instructions on how capabilities of the package should be deployed. The declarative nature of the package means everything is represented by code and automatically runs as it is configured, instead of having to give manual steps that might not be reproducible on all systems.

Zarf Packages are made up of functionality blocks called components which are described more in the [Zarf Components page](./zarf-components). These components can be optional, giving more flexibility to how packages can be used.

<br />

<!-- TODO: @JPERRY This feels out of place here.. -->

## Deploying on to Airgapped Systems

Zarf packages are built with all the dependencies necessary being included within the package itself, this is important when deploying on to systems. Since there is no need for an outbound connection to the internet, these packages become highly distributable and can be run on edge, embedded systems, secure cloud, data centers, or even in a local environment. When deploying a package onto a cluster, the dependencies of the cluster (which were included in the package itself when it was created) are pushed into a docker registry and git server that Zarf stands up on the airgapped system. This way later steps can use the dependencies as they are needed.

<br />
<br />

## Types of Zarf Packages

There are two types of Zarf packages, a `ZarfInitConfig` and a `ZarfPackageConfig`. The package type is defined by the `kind:` field in the zarf.yaml file.

For the remainder of the docs, we will often refer to the `ZarfInitConfig` as an `init config` or `init package` package and the `ZarfPackageConfig` as any `package`.

### ZarfInitConfig

The init package is the package you use to initialize your cluster to be ready to deploy other Zarf packages on to. Because the init package is a special package, we have more documentation in the [Zarf 'init' Package page](./the-zarf-init-package) if you're still curious after reading this section.

**The init package needs to be run once on every cluster you want to deploy another package onto, even if the clusters share the same host.**

If you don't have a cluster running yet, the init-package can help with that too! The init-package has a deployable k3s cluster as a component that can optionally be deployed onto your machine. An init-package will almost always be the first Zarf package you deploy onto a cluster since other packages will often depend on the services the package installs onto your cluster.

> Note: The only exception where you wouldn't deploy an init package first is when you don't have a k8s cluster yet, you don't want to deploy with the k3s distro built into the init-package and you have a package that deploys your preferred distro. In those situations, you can deploy the distro package first, then the init-package, and then whatever other packages you want.)

While initializing, Zarf will seed your cluster with an container registry so it can have a place to push images that other packages will need. The init package will also optionally deploy other functionality to your cluster, such as a git-server to push git repositories to, or a simple PLG logging stack so you can monitor the things running on your cluster.

#### Using the init-package

You initialize your cluster by running the command `zarf init`, which will search your current working directory for a file that matches the name `zarf-init-{ARCHITECTURE}.tar.zst` where the `ARCHITECTURE` matches the architecture of the host you are running on. If the machine your are deploying onto has a different machine architecture, you will have to specify the name of the architecture you are deploying onto. For example, if you are on a arm64 machine but are deploying on a amd64 machine, you will run `zarf init zarf-init-amd64.tar.zst`

At the end of the day, init packages are just like other packages, meaning they can also be run with `zarf package deploy zarf-init-{ARCHITECTURE}.tar.zst`

Init configs are not something you will have to create yourself unless you really want to customize how your cluster is installed / configured (i.e. if you wanted to use the init process to install a specifically configured k3s cluster onto your host machine), and even then it is often easier to create a specific package to do that before your run the init package.

### ZarfPackageConfig

`ZarfPackageConfigs` is any package that isn't an init package. These packages define named capabilities that you want to deploy onto your already initialized cluster.

You can deploy a Zarf package with the command `zarf package deploy` which will bring up a prompt listing all of the files in your current path that match the name `zarf-package-*.tar.zst` so that you can select which package you want to deploy. If you already know which package you want to deploy, you can do that easily with the command `zarf package deploy {PACKAGE_NAME}`.

When Zarf is deploying the package, it will use the infrastructure that was created when doing the 'init' process (such as the docker registry and git server) to push all of the images and repos that the package needs to operate.

<br />
<br />

## What Makes Up A Package

Zarf packages are split into smaller chunks called 'components'. These components are defined more in the [Zarf Components page](./zarf-components) but a quick way to understand components are as the actual named capabilities that packages provide. The schema of a zarf.yaml package follows the following format:

```yaml
kind: <STRING> # Either ZarfPackageConfig or ZarfInitConfig
metadata:
  name: <STRING> # The name of the package
  description: <STRING> # A description of the package
seed: <STRING> # Docker registry to seed the cluster with. Only used for init packages
components: <OBJ LIST> # Components definitions are complex and broken down more in the 'Understanding Zarf Components' page
```

<br />
<br />

## Building A Zarf Package


:::info

**Dependencies** for Building a Zarf Package

- A local k8s cluster to work with ([k3s](https://k3s.io/)/[k3d](https://k3d.io/v5.4.1/)/[KinD](https://kind.sigs.k8s.io/docs/user/quick-start#installation))
- A Zarf CLI ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../the-zarf-cli/building-your-own-cli))
- A Zarf init package ([downloaded](https://github.com/defenseunicorns/zarf/releases) or [manually built](../the-zarf-cli/building-your-own-cli))

:::

The process of defining a package is covered in the [Creating Your Own Package](https://google.com) page. Assuming you have a package already defined, building the package itself is fairly simple.

`zarf package create` will look for a `zarf.yaml` file in the current directory and build the package from that file. Behind the scenes, this is pulling down all the resources it needs from the internet and placing them in a temporary directory, once all the necessary resources of retrieved, Zarf will create the tarball of the temp directory and clean up the temp directory.

<br />
<br />

## Inspecting a Built Package

`zarf package inspect ./path/to/package.tar.zst` will look at the contents of the package and print out the contents of the zarf.yaml file that defined it.
