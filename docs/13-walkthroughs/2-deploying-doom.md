# Deploying Packages

In this walkthrough, we are going to deploy a fun application onto your cluster. In previous walkthroughs, we learned how to [create a package](./0-using-zarf-package-create.md) and [initialize a cluster](./1-initializing-a-k8s-cluster.md). We will be leveraging all that past work and then go the extra step of deploying an application onto our cluster with the `zarf package deploy` command. While this example game is nothing crazy, this walkthrough hopes to show how simple it is to deploy packages of functionality into a Kubernetes cluster.

## Walkthrough Prerequisites

1. The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([`git clone` Instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
2. Zarf binary installed on your $PATH: ([Installing Zarf](../3-getting-started.md#installing-zarf))
3. A Kubernetes cluster initialized by Zarf: ([Initializing a Cluster](./1-initializing-a-k8s-cluster.md))
4. The [game package we created](./0-using-zarf-package-create.md).

## Deploying The Games

``` bash
cd zarf                   # Enter the zarf repository that you have cloned down

zarf package deploy       # Deploy the games package
                          # NOTE: Since we are not providing the path to the package as an argument, we will enter that when prompted
                          # Select the dos-game package
                          # Type `y` when prompted and then hit the enter key
```

### Selecting the Games Package

Since we did not provide the path to the package as an argument to the `zarf package deploy` command, Zarf will prompt you asking for you to choose which package you want to deploy. There is a useful tab-suggestions feature that makes selecting between different packages in your directories easier.

![Package Deploy Selection Tab](../.images/walkthroughs/package_deploy_tab.png)
By hitting 'tab', you can use the arrow keys to select which package you want to deploy. Since we are deploying the games package in this walkthrough, we will select that package and hit 'enter'.

![Package Deploy Tab Selection](../.images/walkthroughs/package_deploy_tab_selection.png)
As we have seen a few times now, we are going to be prompted with a confirmation dialog asking us to confirm that we want to deploy this package onto our cluster.v

### Connecting to the Games

When the games package finishes deploying, you should get an output that lists a couple of new commands that you can use to connect to the games. These new commands were defined by the creators of the games package to make it easier to access the games. By typing the new command, your browser should automatically open up and connect to the application we just deployed into the cluster.
![Connecting to the Games](../.images/walkthroughs/game_connect_commands.png)

``` bash
zarf connect games
```

![Connected to the Games](../.images/walkthroughs/games_connected.png)

:::note
If your browser doesn't automatically open up, you can manually go to your browser and copy the IP address that the command printed out into the URL bar.
:::

:::note
The `zarf connect games` will continue running in the background until you close the connection by pressing the `control + c` in your terminal to terminate the process.
:::

## Credits

:sparkles: Special thanks to these fine references! :sparkles:

- <https://www.reddit.com/r/programming/comments/nap4pt/dos_gaming_in_docker/>
- <https://earthly.dev/blog/dos-gaming-in-docker/>
