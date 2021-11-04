# Zarf Components - Add Logging

This example demonstrates using a [Zarf component](./components.md) to inject zero-config, centralized logging into your Zarf cluster.

More specifically, you'll be adding a [Promtail / Loki / Grafana (PLG)](https://github.com/grafana/loki) stack to the example game cluster by installing Zarf's "logging" component.

&nbsp;


## The Flow

<a href="https://asciinema.org/a/446956?x-scenario=examples-game-logging&autoplay=1">
<img align="right" alt="asciicast" src="https://asciinema.org/a/446956.svg?x-scenario=examples-game-logging" height="320" />
</a>

Here's what you'll do in this example:

1. [Get ready](#get-ready)

1. [Install the logging component](#install-the-logging-component)

1. [Note the credentials](#note-the-credentials)

1. [Check the logs](#check-the-logs)

1. [Cleanup](#cleanup)

&nbsp;

&nbsp;


## Get ready

<a href="https://asciinema.org/a/446956?x-scenario=examples-game-logging&t=1">
<img align="right" alt="asciicast" src="https://asciinema.org/a/446956.svg?x-scenario=examples-game-logging" height="256" />
</a>

This scenario builds upon the previous one, so:

1. Run through the [Zarf game example](./README.md) again but _**don't** do the cleanup step_ &mdash; you're setup correctly once you can pull the game up in your browser.

1. Take a deep breath&mdash;because it's good for your body&mdash;and  read on!

&nbsp;

&nbsp;


## Install the logging component

<a href="https://asciinema.org/a/446956?x-scenario=examples-game-logging&t=19">
<img align="right" alt="asciicast" src="https://asciinema.org/a/446956.svg?x-scenario=examples-game-logging" height="256" />
</a>

Installing a Zarf component is _really_ easy&mdash;you just have to let `zarf init` know that you want use it.  That's it!

Exactly like when you first created the game example cluster, you _move into the directory holding your init package_ and run:

```sh
cd <same dir as zarf-init.tar.zst>
zarf init
```

You can answer the follow-on prompts in almost the exact same way as during your original install _**except** this time answer "yes" when asked whether to install the "logging" component_.

Give it some time for the new logging pods to come up and you're ready to go!

 > _**Note**_
 >
 > You can install components as part of new cluster installs too (obviously)&mdash;there's no need to update afterward if you already know you need a component.

 > _**Note**_
 >
 > Zarf supports non-interactive installs too! See `zarf init --help` for how to make that work.

&nbsp;


## Note the credentials

Go back to your terminal and review the `zarf init` command output&mdash;the very last thing printed should be a set of credentials Zarf has generated for you.

Pay attention to these because you're going to need them to log into your shiny, new [Grafana](https://grafana.com/docs/) installation.

The line you want will look something like this:

```sh
WARN[0026] Credentials stored in ~/.git-credentials      Gitea Username (if installed)=zarf-git-user Grafana Username=zarf-admin Password (all)="AbCDe0fGH12IJklMnOPQRSt~uVWx"
```

Pull out the `Grafana Username` and `Password (all)` values & save them for later.

&nbsp;


## Check the logs

<a href="https://asciinema.org/a/446956?x-scenario=examples-game-logging&t=55">
<img align="right" alt="asciicast" src="https://asciinema.org/a/446956.svg?x-scenario=examples-game-logging" height="256" />
</a>

We've only _just_ installed the logging utilities so we (likely) haven't had time to record anything interesting. Since log aggregation & monitoring aren't worth much without something to collect, let's get some data in there.

&nbsp;


### Generate some traffic

Pull up the game in your brower&mdash;_[instructions here](./README.md#space-marine-the-demon-invasion), in case you forgot how_&mdash;and then reload the browser window a few times.

Doing that sends a bunch of HTTP traffic into the cluster & should give you something worth looking at in Grafana.

&nbsp;


### Get into Grafana

<a href="../../.images/get-started/plg.png">
<img align="right" alt="dosbox" src="../../.images/get-started/plg.png" height="160" />
</a>

Now that you've got some logs worth looking at, you're ready to log into your brand new Grafana instance.

Get started by navigating your browser to: `https://localhost/monitor/explore`.

You'll be redirected the `/login` page where you have to sign in with the Grafana credentials you saved [in a previous step](#note-the-credentials).

Once you've successfully logged in you will be redirected back to:

1. the `monitor/explore` page, where

1. you can select `Loki` in the dropdown, and then

1. enter `{app="game"}` into the Log Browser query input field

Submit that query and you'll get back a dump of all the game pod logs that Loki has collected. Neat!

&nbsp;


## Cleanup

<a href="https://asciinema.org/a/446956?x-scenario=examples-game-logging&t=88">
<img align="right" alt="asciicast" src="https://asciinema.org/a/446956.svg?x-scenario=examples-game-logging" height="256" />
</a>

Once you've had your fun it's time to clean up.

In this case, since the Zarf cluster was installed specifically (and _only_) to serve this example, clean up is really easy&mdash;you just tear down the entire cluster:

```sh
zarf destroy --confirm
```

It takes just a couple moments for the _entire Zarf cluster_ to disappear&mdash;long-running system services and all&mdash;leaving your machine squeaky clean.
