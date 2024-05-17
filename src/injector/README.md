
# zarf-injector

> If using VSCode w/ the official Rust extension, make sure to open a new window in the `src/injector` directory to make `rust-analyzer` happy.
>
> ```bash
> code src/injector
> ```

A tiny (<1MiB) binary statically-linked with musl in order to fit as a configmap.

See how it gets used during the [`zarf-init`](https://docs.zarf.dev/commands/zarf_init/) process in the ['init' package reference documentation](https://docs.zarf.dev/ref/init-package/).

## What does it do?

```sh
zarf-injector <SHA256>
```

The `zarf-injector` binary serves 2 purposes during 'init'.

1. It re-assembles a multi-part tarball that was split into multiple ConfigMap entries (located at `./zarf-payload-*`) back into `payload.tar.gz`, then extracts it to the `/zarf-seed` directory. It also checks that the SHA256 hash of the re-assembled tarball matches the first (and only) argument provided to the binary.
2. It runs a pull-only, insecure, HTTP OCI compliant registry server on port 5000 that serves the contents of the `/zarf-seed` directory (which is of the OCI layout format).

This enables a distro-agnostic way to inject real `registry:2` image into a running cluster, thereby enabling air-gapped deployments.

## Building in Docker (recommended)

```bash
make build-with-docker
```

## Building on Debian-based Systems

Install [Rust](https://rustup.rs/) and `build-essential`.

```bash
make build-injector-linux list-sizes
```

## Building on Debian-based Systems

Install [Rust](https://rustup.rs/) and `build-essential`.

```bash
make build-injector-linux list-sizes
```

## Building on Apple Silicon 

* Install Cross
* Install Docker & have it running
* Rust must be installed via Rustup (Check `which rustc` if you're unsure)

```
cargo install cross --git https://github.com/cross-rs/cross
```

Whichever arch. of `musl` used, add to toolchain
```
rustup toolchain install --force-non-host stable-x86_64-unknown-linux-musl
```
```
cross build --target x86_64-unknown-linux-musl --release

cross build --target aarch64-unknown-linux-musl --release
```

This will build into `target/*--unknown-linux-musl/release`



## Checking Binary Size

Due to the ConfigMap size limit (1MiB for binary data), we need to make sure the binary is small enough to fit.

```bash
make list-sizes
```

```sh
$ make build-with-docker
...

Size of Zarf injector binaries:

840k    target/x86_64-unknown-linux-musl/release/zarf-injector
713k    target/aarch64-unknown-linux-musl/release/zarf-injector
```

```sh
$ make build-with-docker
...

Size of Zarf injector binaries:

840k    target/x86_64-unknown-linux-musl/release/zarf-injector
713k    target/aarch64-unknown-linux-musl/release/zarf-injector
```

## Testing your injector

Build your injector by following the steps above, or running one of the following:
```
make build-injector-linux

## OR 
## works on apple silicon 
make cross-injector-linux 

```

Point the [zarf-registry/zarf.yaml](../../packages/zarf-registry/zarf.yaml) to
the locally built injector image.

```
    files:
      # Rust Injector Binary
      - source: ../../src/injector/target/x86_64-unknown-linux-musl/release/zarf-injector
        target: "###ZARF_TEMP###/zarf-injector"
        <!-- shasum: "###ZARF_PKG_TMPL_INJECTOR_AMD64_SHASUM###" -->
        executable: true

    files:
      # Rust Injector Binary
      - source: ../../src/injector/target/aarch64-unknown-linux-musl/release/zarf-injector
        target: "###ZARF_TEMP###/zarf-injector"
        <!-- shasum: "###ZARF_PKG_TMPL_INJECTOR_ARM64_SHASUM###" -->
        executable: true
```

In Zarf Root Directory, run:
```
zarf tools clear-cache
make clean
make && make init-package
```

If you are running on an Apple Silicon, add the `ARCH` flag:  `make init-package ARCH=arm64`

This builds all artifacts within the `/build` directory. Running `zarf init` would look like:
`.build/zarf-mac-apple init --components git-server -l trace`
