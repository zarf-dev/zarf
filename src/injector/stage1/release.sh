#!/usr/bin/env bash

cargo build --target x86_64-unknown-linux-musl --release

cargo build --target aarch64-unknown-linux-musl --release

size_linux=$(du --si target/x86_64-unknown-linux-musl/release/zarf-injector | cut -f1)
echo "Linux binary size: $size_linux"
size_aarch64=$(du --si target/aarch64-unknown-linux-musl/release/zarf-injector | cut -f1)
echo "aarch64 binary size: $size_aarch64"
