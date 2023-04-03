# Building Your Own Zarf CLI

To build the Zarf CLI from scratch, you'll need to have the following dependencies configured:

1. The Zarf repository cloned down:
   - `git clone git@github.com:defenseunicorns/zarf.git`.
2. Have Go 1.19.x installed on your PATH (instructions to [install Go](https://go.dev/doc/install)).
3. Have NPM/Node 18.x installed on your PATH (instructions to [install NPM/Node](https://nodejs.org/en)).
4. `make` utility installed on your PATH.
   - Instructions to install on macOS with [Homebrew](https://formulae.brew.sh/formula/make).
   - Instructions to install on Windows with [Chocolatey](https://community.chocolatey.org/packages/make), [Scoop](https://scoop.sh/#/apps?q=make&s=0&d=1&o=true&id=c43ff861c0f1713336e5304d85334a29ffb86317), or [MSYS2](https://packages.msys2.org/package/make).

:::note

If you are running `make` targets other than the `build-cli-*` targets described below, you may need more software installed.  Inspect the `Makefile` at the root of the project to view the commands each target runs.

:::


If you don't want to build the CLI yourself, you can download a pre-compiled binary from the 'Assets' tab of our latest [releases](https://github.com/defenseunicorns/zarf/releases) on GitHub. The pre-compiled binary is available for both arm64 and amd64 machines. 

## Building the CLI

Once you have the dependencies configured, you can build the Zarf CLI by running the following commands:

```bash
cd zarf        # go into the root level of the zarf repository

make build-cli # This will build binaries for linux, M1 Mac, and Intel Mac machines
               # This puts the built binaries in the ./build directory
```

:::note Optimization Note
The `make build-cli` command builds a binary for each combination of OS and architecture, which may take some time. If you only need a binary for a specific configuration, you can use one of the following commands:

- `make build-cli-mac-intel`
- `make build-cli-mac-apple`
- `make build-cli-linux-amd`
- `make build-cli-linux-arm`
- `make build-cli-windows-amd` 
- `make build-cli-windows-arm`
:::

#### The Technical Process

[Under the hood](https://github.com/defenseunicorns/zarf/blob/473cbd5be203bd38254556cf3d55561e5be247dd/Makefile#L44), the `make` command executes a `go build .....` command with specific `CGO_ENABLED`, `GOOS`, and `GOARCH` flags depending on the distro and architecture of the system it is building for. The `CLI_VERSION` is passed in as a `ldflag` and is set the latest tag is in the repository as defined by `git describe --tags`.
