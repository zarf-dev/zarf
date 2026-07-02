# squashfs

[![PkgGoDev](https://pkg.go.dev/badge/github.com/CalebQ42/squashfs)](https://pkg.go.dev/github.com/CalebQ42/squashfs) [![Go Report Card](https://goreportcard.com/badge/github.com/CalebQ42/squashfs)](https://goreportcard.com/report/github.com/CalebQ42/squashfs)

This is a fork of `CalebQ42/squashfs` for the purpose of maintaing a package that removes the lzo dependency, so that it does not contain GPL code.

## Branches

* `remove-lzo` - `main` from `CalebQ42/squashfs` with LZO support removed.
* `remove-lzo-vX.Y.Z` - `vX.Y.Z` from `CalebQ42/squashfs` with LZO support removed.

## Tags

* `vX.Y.Z` - `vX.Y.Z` from `CalebQ42/squashfs` with LZO support removed.

-----

A PURE Go library to read squashfs. There is currently no plans to add archive creation support as it will almost always be better to just call `mksquashfs`. I could see some possible use cases, but probably won't spend time on it unless it's requested (open a discussion if you want this feature).

The library has two parts with this `github.com/CalebQ42/squashfs` being easy to use as it implements `io/fs` interfaces and doesn't expose unnecessary information. 95% this is the library you want. If you need lower level access to the information, use `github.com/CalebQ42/squashfs/low` where far more information is exposed.

Currently has support for reading squashfs files and extracting files and folders.

Special thanks to <https://dr-emann.github.io/squashfs/> for some VERY important information in an easy to understand format.
Thanks also to [distri's squashfs library](https://github.com/distr1/distri/tree/master/internal/squashfs) as I referenced it to figure some things out (and double check others).

## FUSE

As of `v1.0`, FUSE capabilities has been moved to [a separate library](https://github.com/CalebQ42/squashfuse).

## Limitations

* No Xattr parsing.
* Socket files are not extracted.
  * From my research, it seems like a socket file would be useless if it could be created.
* Fifo files are ignored on `darwin`

## Issues

* Significantly slower then `unsquashfs` when nested images
  * This seems to be related to above along with the general optimization of `unsquashfs` and it's compression libraries.
    * Not to mention it's written in C
  * Times seem to be largely dependent on file tree size and compression type.
    * My main testing image (~100MB) using Zstd takes about 5x longer.
    * An Arch Linux airootfs image (~780MB) using XZ compression with LZMA filters takes about 30x longer.
    * A Tensorflow docker image (~3.3GB) using Zstd takes about 12x longer.

Note: These numbers are using `FastOptions()`. `DefaultOptions()` takes about 2x longer.

## Recommendations on Usage

Due to the above performance consideration, this library should only be used to access files within the archive without extraction, or to mount it via Fuse.

* Neither of these use cases are largely effected by the issue above.
