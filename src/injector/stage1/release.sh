#!/usr/bin/env bash

# first follow steps from: https://github.com/johnthagen/min-sized-rust
# and: https://github.com/johnthagen/min-sized-rust/issues/45

cargo +nightly build -Z build-std=std,panic_abort -Z build-std-features=panic_immediate_abort \
    --target x86_64-unknown-linux-musl --release

cargo +nightly build -Z build-std=std,panic_abort -Z build-std-features=panic_immediate_abort \
    --target aarch64-apple-darwin --release

cargo +nightly build -Z build-std=std,panic_abort -Z build-std-features=panic_immediate_abort \
    --target aarch64-unknown-linux-musl --release

size_linux=$(du --si target/x86_64-unknown-linux-musl/release/zarf-injector | cut -f1)
echo "Linux binary size: $size_linux"
size_m1=$(du --si target/aarch64-apple-darwin/release/zarf-injector | cut -f1)
echo "M1 Mac binary size: $size_m1"
size_aarch64=$(du --si target/aarch64-unknown-linux-musl/release/zarf-injector | cut -f1)
echo "aarch64 binary size: $size_aarch64"
