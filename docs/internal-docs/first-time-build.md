# Build Your First Zarf

Creating a Zarf release is a two-stage process:
1. Build the zarf _binaries_.
1. Build the zarf _distribution package_.

Usually (during development) these two stages are accomplished with a single command, but doing so requires that you _already_ have a copy of zarf installed; that won't be the case **the first time** you try to build Zarf on a [fresh, development workstation](./workstation.md#i-need-a-dev-machine).

To get a new workstation setup for that single-command workflow, see below!

&nbsp;


## 1. Build the CLI

The first step toward Zarf development is to getting yourself a set of zarf binaries.  You can build them like so:

```sh
make build-cli
```
This command creates a `./build` directory and dumps the various zarf binaries into it.

&nbsp;


## 2. Log into Iron Bank

Zarf distribution packages are built on top of images from Platform One's container image repository, Iron Bank. To access them, you'll need to 1) secure Iron Bank pull credentials, and 2) configure Zarf to use them.

To use your new Zarf binaries to authenticate with Iron Bank:

1. Move into the binary build directory:

    ```sh
    cd ./build
    ```

2. Follow the instructions you find here: [Zarf Login](./ironbank.md#zarf-login).

    > _**Take note**_
    >
    > Your credentials are cached in a file on your development machine so you should only need to do this login step _once_!

3. Finally, move back "up" to the project root:

    ```sh
    cd ..
    ```

And with that, you're ready to build your first Zarf distribution package.

&nbsp;


## 3. Build the distribution package

The command to build your first Zarf distribution package looks like this:

```sh
make init-package
```

> _**Build failed?**_
>
> If your build fails with a message like this:
> ```sh
> WARN[0010] Unable to pull the image   image="registry1.dso.mil/[...]"
> ```
> It is likely that you've forgotten to setup access to Iron Bank _or_ that your credentials have changed. In either case, you should go back through the steps to [Log into Iron Bank](#2-log-into-iron-bank) & try to build again!

Assuming everything works out, you should see a shiny new `zarf-init-<arch>.tar.zst` in your `./build` directory.

Congratulations!  You've just built yourself a Zarf!

&nbsp;


## &#8734;. Happily ever after

After you've worked your way through steps 1-3 above you can use a simpler, single command to create a Zarf release:

```sh
make build-test
```

This will use the credentials you established [above](#2-log-into-iron-bank) to dump a fresh set of binaries + distribution package into your `./build` directory&mdash;an easy to use, simple to remember tool for your Zarf dev & test toolbox!

&nbsp;


## Next steps

Now that you can build _your own_ Zarf, it's a great time to try using it to run our [Get Started - game](../examples/game/) example!
