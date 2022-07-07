# Creating a Zarf Package
<!-- Does the word 'Creating' make it seem like you're 'defining' the package vs running the 'zarf package create' command? -->

In this walkthrough we will be demonstrating how to build a Zarf package. In fact, we will be building two packages: the first will be the zarf-init package (which will be very useful for nearly every other walkthrough) and the second will be a package that contains a handful of legacy DOS games that we will be using in the [deploying doom](./deploying-doom) walkthrough later. 

When creating a Zarf package, you will need to have internet connection out so that Zarf can fetch all the dependencies and resources necessary to build the package. If your package is using images from a private registry or is referencing repositories in a private repository, you will need to have your credentials configured on your machine in order for Zarf to be able to fetch the resources.


## Walkthrough Prequisites
1. The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([git clone instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
2. Zarf binary installed on your $PATH: ([install instructions](../getting-started#installing-zarf))



## Building the init-package
Creating zarf packages is a simple process that can be completed in a single command; [`zarf package create`](../user-guide/the-zarf-cli/cli-commands/package/zarf_package_create). This command looks for a `zarf.yaml` file in the current directory and creates a package containing all the resources the file defines. More information about what a Zarf package is can be found on the [Zarf Packages](../user-guide/zarf-packages/zarf-packages) page.

```bash
cd zarf                   # Enter the zarf repository that you have cloned down

zarf package create .     # Run the command to create the zarf package
                          # Type `y` when prompted and then hit the enter key
```
This set of commands will create a zarf package in the current directory. In this case, the package name should look something like `zarf-init-amd64.tar.zst`, although it might be slightly different depending on your system architecture.

<br />

When you execute the `zarf package create` command, Zarf will prompt you to confirm that you want to create the package by printout out the package definition and asking you to respond with either `y` or `n`.
![Confirm Package Creation](../../static/img/walkthroughs/package_create_confirm.png)
:::info
You can skip this confirmation by adding the `--confirm` flag when running the command.
This will look like: `zarf package create . --confirm`
:::

<br />
<br />


## Building the game package
<!-- TODO: After PR #511 gets merged maybe we should change this to path to the directory through the command instead of explicitly doing a 'cd' -->
<!-- https://github.com/defenseunicorns/zarf/pull/511 -->
Creating the game package is just as simple as creating the init-package!  Once again, we will be using the `zarf package create` command to create the package. Since the game package definition lives in `examples/game` within the Zarf repository the only thing we NEED to do differently than before is make sure we are in the correct directory when we execute the command. While we don't NEED to, when executing the command this time, we will skip the confirmation prompt by adding the `--confirm` flag just to save a bit of time/keystrokes.

```bash
cd examples/game
zarf package create . --confirm
```
This set of commands will create a zarf package in the current directory. In this case, the package name should look something like `zarf-package-dos-games-amd64.tar.zst`, although it might be slightly different depending on your system architecture.


