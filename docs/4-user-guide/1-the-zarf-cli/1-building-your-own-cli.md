# Building Your Own Zarf CLI

:::note
As mentioned in the [Getting Started page](../../getting-started), a pre-compiled binary is available for arm64 and amd64 machines under the 'Assets' tab of our latest releases on [GitHub](https://github.com/defenseunicorns/zarf/releases). If you don't want to build the CLI yourself you could always download it from there.
:::

## Dependencies

:::info

If you want to built the CLI from scratch, you can easily do that! In order to build the cli you will need to make sure you have the following dependencies correctly configured:

1. The Zarf repository cloned down:
   - `git clone git@github.com:defenseunicorns/zarf.git`
2. Have Go 1.18.x installed on your PATH (instructions can be found [here](https://go.dev/doc/install))
3. `make` utility installed on your PATH (instructions to install w/ Homebrew can be found [here](https://formulae.brew.sh/formula/make))

:::

## Building The CLI

Once you have the dependencies configured you can build the Zarf CLI by running the following commands:

```bash
cd zarf        # go into the root level of the zarf repository

make build-cli # This will build binaries for linux, M1 Mac, and Intel Mac machines
               # This puts the built binaries in the ./build directory
```

:::note Optimization Note
The `make build-cli` command builds a binary for each combinations of OS and architecture. If you want to shorten the build time, you can use an alternative command to only build the binary you need:

- `make build-cli-mac-intel`
- `make build-cli-mac-apple`
- `make build-cli-linux-amd`
- `make build-cli-linux-arm`
  :::

#### Breaking Down Whats Happening

[Under the hood](https://github.com/defenseunicorns/zarf/blob/473cbd5be203bd38254556cf3d55561e5be247dd/Makefile#L44), the make command is executing a `go build .....` command with specific `CGO_ENABLED`, `GOOS`, and `GOARCH` flags depending on the distro and architecture of the system it is building for. The `CLI_VERSION` is passed in as a `ldflag` and is set to whatever the latest tag is in the repository as defined by `git describe --tags`.
