# zarf-injector

A tiny (<1MiB) binary statically-linked with musl in order to fit as a configmap.

See how it gets used during the [`zarf-init`](https://docs.zarf.dev/commands/zarf_init/) process in the ['init' package reference documentation](https://docs.zarf.dev/ref/init-package/).

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
