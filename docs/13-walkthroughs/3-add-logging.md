# Add Logging

In this walkthrough, we are going to show how you can use a Zarf component to inject zero-config, centralized logging into your Zarf cluster.

More specifically, you'll be adding a [Promtail / Loki / Grafana (PLG)](https://github.com/grafana/loki) stack to the [Doom Walkthrough](./2-deploying-doom.md) by installing Zarf's "logging" component.


## Walkthrough Prerequisites
1. The [Zarf](https://github.com/defenseunicorns/zarf) repository cloned: ([`git clone` Instructions](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository))
1. Zarf binary installed on your $PATH: ([Install Instructions](../3-getting-started.md#installing-zarf))


## Install the logging component

To install the logging component, follow the [Initializing a Cluster Instructions](./1-initializing-a-k8s-cluster.md), but instead answer `y` when asked to install the `logging` component


## Note the credentials

Review the `zarf init` command output for the following:

![logging-creds](../.images/walkthroughs/logging_credentials.png)

You should see a section for `Logging`.  You will need these credentials later on.


## Deploy the Doom Walkthrough

Follow the remainder of the [Doom Walkthrough](./2-deploying-doom.md).


## Check the logs

:::note

Because Doom is freshly installed it is recommended to refresh the page a few times to generate more log traffic to view in Grafana

:::


### Log into Grafana

To open Grafana you can use the `zarf connect logging` command.

You'll be redirected the `/login` page where you have to sign in with the Grafana credentials you saved [in a previous step](#note-the-credentials).

Once you've successfully logged in go to:

1. The "Explore" page (Button on the left that looks like a compass)

1. you can select `Loki` in the dropdown, and then

1. enter `{app="game"}` into the Log Browser query input field

Submit that query and you'll get back a dump of all the game pod logs that Loki has collected.
