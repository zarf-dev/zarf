*These docs are for Zarf development, not usage.  For just using Zarf, go to [main website](https://www.zarf.dev)*

# Building Zarf Assets

Creating a Zarf release is a two-stage process, these assets are part of a normal [Zarf Release](https://github.com/defenseunicorns/zarf/releases), but in dev we'll need to build our own.

1. Build the Zarf binary.
1. Create the zarf init package.
1. Create the Zarf example packages (optional)

```sh
# Purge existing build assets and re-create the packages
make clean init-package build-examples ARCH=<amd64 | arm64>
```

See the Makefile for additional make targets for when only needing to rebuild part of the assets above.

&nbsp;

## Log into Iron Bank (optional)

Some Zarf packages are built on top of images from [Platform One's](https://p1.dso.mil) container image repository, Iron Bank. To access them, you'll need to 1) secure Iron Bank pull credentials, and 2) configure Zarf to use them.

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
