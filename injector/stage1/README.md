## Compile must be statically with musl in order to fit as a configmap
```
CC=/usr/bin/musl-gcc cargo build --release --target=x86_64-unknown-linux-musl
strip target/x86_64-unknown-linux-musl/release/zarf-injector
```
