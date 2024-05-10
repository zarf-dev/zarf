
# zarf-injector

A tiny (<1MiB) binary statically-linked with musl in order to fit as a configmap

## Building on Ubuntu

```bash
# install rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path
source $HOME/.cargo/env

# install build-essential
sudo apt install build-essential -y

# build w/ musl
rustup target add x86_64-unknown-linux-musl
cargo build --target x86_64-unknown-linux-musl --release
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
cargo build --target x86_64-unknown-linux-musl --release

cargo build --target aarch64-unknown-linux-musl --release

size_linux=$(du --si target/x86_64-unknown-linux-musl/release/zarf-injector | cut -f1)
echo "Linux binary size: $size_linux"
size_aarch64=$(du --si target/aarch64-unknown-linux-musl/release/zarf-injector | cut -f1)
echo "aarch64 binary size: $size_aarch64"
```

## Testing your injector

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

In Zarf Root Directory, run `make init-package`, if you are running on an Apple Silicon
be sure run `make init-package --ARCH=arm64`

The init package will be built in `./build`. Take the name of the locally
built init package (e.g. `zarf-init-arm64-v0.33.2-29-g139d8390.tar.zst`)
and change the value of `initPackageName` in the [cmd/initalize](../../src/cmd/initialize.go)

```
--- 		initPackageName := sources.GetInitPackageName()
+++ 		initPackageName := "zarf-init-arm64-v0.33.2-29-g139d8390.tar.zst"
```

Run `make build` in Zarf Root Directory. That will build a local version of 
Zarf in `/build`. You can now use it within the `/build` directory like such:
`./zarf-mac-apple init --components git-server -l trace`

