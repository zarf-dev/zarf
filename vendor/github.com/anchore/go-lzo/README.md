# go-lzo

This repository provides an implementation of the LZO1X decompression algorithm in Go. 
The implementation is derived from the [Linux kernel documentation for the LZO stream format](https://docs.kernel.org/staging/lzo.html) and the [implementation from `lzokay` project](https://github.com/AxioDL/lzokay) (MIT licensed). It includes a `Reader` for streaming decompressed data and a standalone `Decompress` function for use with byte slices.

To use this library:

```bash
go get github.com/anchore/go-lzo
```
