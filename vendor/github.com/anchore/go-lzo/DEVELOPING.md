# Developing

## Getting started

In order to test and develop in this repo you will need the following dependencies installed:
- golang 1.24+
- either docker or the lzo C lib & g++

To run tests and linters:
- `make unit`
- `make lint-fix`

Tests depend on either docker (slower but more portable) or having the LZO c lib + headers as well as the g++ toolchain installed (less portable).
The test utilities will attempt to build the native LZO wrapper tool to `testdata/bin`, and if not successful, will fallback to creating a docker image with the same tooling -- there shouldn't be a need to select one or the other specifically.
These utilities are used to create compression golden files in `testdata/cache` used in unit tests.
Dynamic data is also generated within tests, which is saved to `testdata/crash` when a test fails.

## Attribution considerations

Note that the [suite of LZO algorithms](https://github.com/nemequ/lzo/blob/master/doc/LZO.TXT) from the [original project](http://www.oberhumer.com/opensource/lzo/) are licensed under GPL v2+. The current implementation was derived from the [Linux kernel documentation for the LZO stream format](https://docs.kernel.org/staging/lzo.html) and the [implementation from `lzokay` project](https://github.com/AxioDL/lzokay) (MIT licensed).
It is vital that any additional features and fixes are done without referencing the original implementation, any pseudocode describing the original implementation,
or any code that is otherwise licensed without a permissive license.
