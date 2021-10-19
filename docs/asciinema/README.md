# asciinema

The Zarf docs include a series of recorded / published terminal sessions which we use to brag to (potential) users about the appeal of our clear, easy-to-use CLI. These recordings let them _experience_ Zarf in a way that simply reading can't, they pass on pro usage tips / context, and they help new users find their footing with the tool&mdash;we love them.

They don't come "for free" though&mdash;creating them takes time; keeping them current takes even more.

To facilitate this process we use a product called [asciinema](https://asciinema.org/) which does two things for us: 1) captures _actual terminal sessions_, and 2) serves as a simple, backing web host for storage & (browser-based) playback of recordings.

Best of all, because it provides for both of those functions through its CLI, we can _script the creation & update of our terminal session recordings_ for ultra-low cost of maintenance & near total elimination of toil. Yay!

&nbsp;


## Prerequisites

Here's what you'll need to generate the project "asciinemas":

- [asciinema](https://asciinema.org/docs/installation) &mdash; A lightweight, purely text-based approach to terminal recording.

- [expect](https://en.wikipedia.org/wiki/Expect) &mdash; A tool for automating interactive, text-based applications (like `asciinema` and `zarf`!) according to a script. Installation instructions are available for lots of OSes (like [ubuntu](https://ubuntu.pkgs.org/20.04/ubuntu-universe-amd64/expect_5.45.4-2build1_amd64.deb.html#download) and [mac](https://stackoverflow.com/a/48880853)).

- [curl](https://curl.se/download.html) &mdash; A command line tool for transferring data with URLs&mdash;think: command line "web browser". Installation instructions are available for most OSes (like [ubuntu](https://linuxize.com/post/how-to-install-and-use-curl-on-ubuntu-20-04/) and [mac](https://formulae.brew.sh/formula/curl)).

&nbsp;

## The Flow

1. [Setup Your Environment](#setup-your-environment)

1. [Run Scenarios](#run-scenarios)

1. [Review Recordings](#review-recordings)

1. [Publish to Asciinema.org](#publish-to-asciinemaorg)

&nbsp;


## Setup Your Environment

1. Put a Zarf release into the `<project root>/build` folder&mdash;it doesn't matter if you download it or build it yourself, just make sure _it's the version you want to generate docs against_.

1. Log Zarf into Iron Bank&mdash;instructions [here](../ironbank.md#2-configure-zarf-the-use-em).

1. Install the [prereqs](#prerequisites) on your system (or just use the [./Vagrantfile](./Vagrantfile) if you already have [the stuff for that](../workstation.md#i-want-a-demoexample-sandbox) installed).

1. Configure your "asciinema<area>.org install id" such that `asciinema` uploads are tied to Zarf's collection of recordings. Follow [these instructions](#setup---zarf-install-id) then come back & continue on.


&nbsp;


## Run Scenarios

Make sure the `expect` scripts you need _exist in the `./scenarios` folder_, then run the utility script:

```sh
./asciinema.sh run [""|"all"]   # run & record ALL scenarios
./asciinema.sh run <filename>   # run & record a SPECIFIC scenario
```

Watch the terminal do its magic & dump your new recordings to the `./recordings` folder.

&nbsp;


## Review Recordings

By this point, you should have some terminal session recordings in the `./recordings` folder. They (currently) only exist on _your machine_ though, so this is your chance to check them before they "go live".

You can use your local terminal _sort-of-like_ a video player & "play" back any of your recordings like so:

```sh
asciinema play <path/to/recording>
```

Be sure to do a review & verify that what you caught during recording was everything you expected to!

&nbsp;


## Publish to Asciinema<area>.org

Once all of your recordings look good it's time to push them out to https://asciinema.org!

During publishing `asciinema` uses a unique identifier&mdash;called an "install id"&mdash;to connect an uploaded recording to the appropriate asciinema.org account. It stores this id in a file on your disk: `$HOME/.config/asciinema/install-id`.

When you run `asciinema` commands, if you do not already have an `install-id` file one is generated for you. This makes for a nice new-user experience and if you only need to upload anonymous, temporary recordings to asciinema.org this is enough.

If you need recordings to stick around and be listed in a specific group, however&mdash;and in this case you do&mdash;then you'll need access to an install id that has been registered with Zarf's account on asciinema.org.

> _**Dig in!**_
> 
> Go ahead and have a look at the asciinema [usage page](https://asciinema.org/docs/usage) (under the "auth" heading) if you're interested in more detail / background on this topic.

&nbsp;

### The Flow

1. [Publish New Recordings](#publish-new-recordings)

1. [Update Project Links](#update-project-links)

1. [Remove Outdated Recordings](#remove-outdated-recordings)

&nbsp;


### Setup - Zarf install id

#### Access

For access to an "install id" that has been authorized to upload recordings to [the Zarf account](./asciinema-org) on asciinema.org, you can:

- Login to [Zarf's asciinema.org account](./asciinema-org) and copy down a "recorder token" (a.k.a. "install id") from the [account settings](https://asciinema.org/user/edit) page, _**or**_&mdash;

- Generate a new install id locally, then login & associate it to [Zarf's asciinema.org account](./asciinema-org) (as described [under the "auth" heading](https://asciinema.org/docs/usage)), _**or**_&mdash;

- Get someone else with access to do any of the above _for_ you & just send back the install id!


&nbsp;


#### Associate

Once you have access to an authorized install id you need to put it in a location on disk where `asciinema` knows to look for it.

**If you are configuring your own machine to run asciinema** you can drop your install id into an appropriate file with a couple of commands that look like this:

```sh
mkdir -p $HOME/.config/asciinema
echo -n "<install id>" > $HOME/.config/asciinema/install-id
```

**If you plan to use the [./Vagrantfile](./Vagrantfile) to run asciinema instead** you should dump your install id into a `./.config/asciinema/install-id` file (in the same directory as the README.md you're reading right now!) as this will allow your install id to be mounted into the VM on startup:

```sh
mkdir -p ./.config/asciinema
echo -n "<install id>" > ./.config/asciinema/install-id
```

&nbsp;


### Publish New Recordings

After you've configured `asciinema` to use the Zarf install id it's time to publish some recordings!

Again, using the utility script, you'll run a command that looks something like this:

```sh
./asciinema.sh pub [""|"all"]   # upload ALL recordings
./asciinema.sh pub <filename>   # upload a SPECIFIC recording
```

> _**Take note**_
>
> Publishing _always creates a brand new link_ at asciinema.org! There is, unfortunately (as of Oct 2021), no "official" way to update a recording using the `asciinema` CLI&mdash;ugh.

Once the recordings have been published you'll see some `<filename>.url` files appear in your `recordings` folder&mdash;these files contain the **new** asciinema<area>.org urls that you'll use to update the rest of your project links.

&nbsp;


### Update Project Links

After you publish recordings to asciinema<area>.org you'll have some project docs' links to update. By-hand updates can be slow, annoying, and error-prone though, so... utility script to the rescue!

```sh
./asciinema.sh see              # see where all the links are (manual use!)
./asciinema.sh sub              # substitute old urls for new (full auto!)
```

These commands will speed you along no matter which manner of update you prefer (manual or auto).

> _**Recommendation**_
>
> Post-modification, pre-commit reviews are less bothersome (and easier to rollback) if you clear your working directory (e.g. `git stash`) before running the `sub` command.

&nbsp;


### Remove Outdated Recordings

The last step in the asciinema<area>.org publishing flow is to go and _clean out old recordings_. Regrettably, this step is a manual one because there is no "official" way (as of Oct 2021) to delete recordings via the `asciinema` CLI.

The task here is pretty simple, thankfully:
1. Point your browser at the asciinema.org account listed in [this file](./asciinema-org).

1. Log in.

1. Delete all recordings _other than the most recent upload of **any given title**_ by handing-off to this utility script command:

    ```sh
    ./asciinema.sh zap      # nukes ALL the expired stuff!
    ```

    Some interaction will be required for this command to complete&mdash;you'll have to dig some cookies out of your browser's dev tools & hand them over. While this isn't an ideal situation, it _is_ about the best we can do without "real" support for deletes in `asciinema`. It's still _way_ better than trying to cleanup a bunch of recordings by hand.
&nbsp;
