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
