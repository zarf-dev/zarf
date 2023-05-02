# Using Zarf Package Create

## Introduction

In this tutorial, we will demonstrate how to build a Zarf package with `zarf package create`. We will build two packages: the first will be the zarf init-package (which will be very useful for nearly every other tutorial) and the second will be a Helm OCI chart package.

When creating a Zarf package, you must have an Internet connection so that Zarf can fetch all of the dependencies and resources necessary to build the package. If your package is using images from a private registry or is referencing repositories in a private repository, you will need to have your credentials configured on your machine for Zarf to be able to fetch the resources.

## System Requirements

- You'll need an internet connection so Zarf can pull in anything required to build the package.

## Prerequisites

Before beginning this tutorial you will need the following:

- The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([git clone instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
-  Zarf binary installed on your $PATH: ([Installing Zarf](../1-getting-started/index.md#installing-zarf))
- The [Docker CLI](https://docs.docker.com/desktop/) installed for building the [`zarf-agent`](../8-faq.md#what-is-the-zarf-agent) image.

## Building the init-package

Creating zarf packages is a simple process you can complete with a single command; [`zarf package create`](../2-the-zarf-cli/100-cli-commands/zarf_package_create.md). This command looks for a `zarf.yaml` file in the specified directory and creates a package containing all the resources the file defines. You can find more information about Zarf packages on the [Zarf Packages](../3-create-a-zarf-package/1-zarf-packages.md) page.

```bash
cd zarf                   # Enter the zarf repository that you have cloned down

zarf package create .     # Run the command to create the zarf package
                          # Type `y` when prompted and then hit the enter key
```

This set of commands will create a Zarf package in the current directory. In this case, the package name should look something like `zarf-init-amd64-v0.24.0.tar.zst`, although it might differ slightly depending on your system architecture.

When you execute the `zarf package create` command, Zarf will prompt you to confirm that you want to create the package by displaying the package definition and asking you to respond with either `y` or `n`.

<iframe src="/docs/tutorials/package_create.html" height="500px" width="100%"></iframe>

:::info
You can skip this confirmation by adding the `--confirm` flag when running the command. This will look like: `zarf package create . --confirm`
:::

After you confirm package creation, you have the option to specify a maximum file size for the package. To disable this feature, enter `0`.

<iframe src="/docs/tutorials/package_create_size.html" height="100px" width="100%"></iframe>

Once you enter your response for the package size, the output that follows will show the package being created.

<iframe src="/docs/tutorials/package_create_components.html" height="500px" width="100%"></iframe>

Congratulations! You've just created your first Zarf package!

## Building the Helm OCI chart package

Creating the Helm OCI chart package is just as simple as creating the init package! However, unlike the init package, the Helm OCI chart package does not require Docker. Once again, we will use the `zarf package create` command to create the package. Since the package definition lives in `examples/helm-oci-chart` within the Zarf repository, the only thing we need to do differently is specify the correct directory. This time we will skip the confirmation prompt by adding the `--confirm` flag to save some time.

```bash
zarf package create examples/helm-oci-chart --confirm
```

This will create a zarf package in the current directory with a package name that looks something like `zarf-package-helm-oci-chart-amd64.tar.zst`, although it might be slightly different depending on your system architecture.

Congratulations! You've built the Helm OCI chart package. Now, let's [deploy it](./2-deploying-zarf-packages.md)!

## Troubleshooting

### Unable to read zarf.yaml file

#### Example

<iframe src="/docs/tutorials/package_create_error.html" height="120px" width="100%"></iframe>

#### Remediation

If you receive this error, you may not be in the correct directory. Double-check where you are in your system and try again once you're in the correct directory with the zarf.yaml file that you're trying to build.
