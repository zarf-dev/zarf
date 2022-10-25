# zarf-injector

A tiny (<512kb) binary statically-linked with musl in order to fit as a configmap

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
strip target/x86_64-unknown-linux-musl/release/zarf-injector
```
