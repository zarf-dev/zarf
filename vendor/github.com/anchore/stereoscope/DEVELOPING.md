# Developing

## Getting started

In order to test and develop in this repo you will need the following dependencies installed:
- Golang
- docker
- make
- podman (for benchmark and integration tests only)
- containerd (for integration tests only)
- skopeo (for integration tests only)

After cloning the following step can help you get setup:
1. run `make bootstrap` to download go mod dependencies, create the `/.tmp` dir, and download helper utilities.
2. run `make help` to view the selection of developer commands in the Makefile

The main make tasks for common static analysis and testing are `lint`, `format`, `lint-fix`, `unit`, and `integration`.

See `make help` for all the current make tasks.

## Background

Stereoscope is a library for reading and manipulating container images. It is capable of parsing multiple image 
sources, providing a single abstraction for interacting with them. Ultimately this provides a squashfs-like 
interface for interacting with image layers as well as a content API for accessing files contained within 
the image.

**Overview of objects:**
- `image.Image`: Once parsed with `image.Read()` this object represents a container image. Consists of a sequence of `image.Layer` objects, a `image.FileCatalog` for accessing files, and `filetree.SearchContext` for searching for files from the squashed representation of the image filesystem. Additionally exposes GGCR `v1.Image` objects for accessing the raw image metadata.
- `image.Layer`: represents a single layer of the image. Consists of a `filetree.FileTree` that represents the raw layer contents, and a `filetree.SearchContext` for searching for files relative to the raw (single layer) filetree as well as the squashed representation of the layer relative to all layers below this one.  Additionally exposes GGCR `v1.Layer` objects for accessing the raw layer metadata.
- `filetree.FileTree`: a tree representing a filesystem. All nodes represent real paths (paths with no link resolution anywhere in the path) and are absolute paths (start with / and contain no relative path elements [e.g. ../ or ./]). This represents the filesystem structure and each node has a reference to the file metadata for that path.
- `file.Reference`: a unique file in the filesystem, identified by an absolute, real path as well as an integer ID (`file.ID`s). These are used to reference concrete nodes in the `filetree.FileTree` and `image.FileCatalog` objects.
- `file.Index`: stores all known `file.Reference` and `file.Metadata`. Entries are indexed with a variety of ways to provide fast access to references and metadata without needing to crawl the tree. This is especially useful for speeding up globbing.
- `image.FileCatalog`:  an image-aware extension of `file.Index` that additionally relates `image.Layers` to `file.IDs` and provides a content API for any files contained within the image (regardless of which layer or squashed representation it exists in). 

### Searching for files

Searching for files is exposed to users in three ways:
- search by file path
- search by file glob
- search by file content MIME type

Searching itself is performed two different ways:
- search the `image.FileCatalog` on the image by a heuristic
- search the `filetree.FileTree` directly

The "best way" to search is automatically determined in the `filetree.searchContext` object, exposed on `image.Image` and `image.Layer` objects as a `filetree.Searcher` for general use.

### File trees

The `filetree.FileTree` object represents a filesystem and consists of `filenode.Node` objects. The tree itself leverages `tree.Tree` as a generic datastructure. What `filetree.FileTree` adds is the concept of file types, the semantics of each type, the ability to resolve links based on a given strategy, merging of trees with the same semantics of a union filesystem (e.g. whiteout files), and the ability to search for files via direct paths or globs. 

The `fs.FS` abstraction has been implemented on `filetree.FileTree` to allow for easy integration with the standard library as well as to interop with the `doublestar` library to facilitate globing. Using the `fs.FS` abstraction for filetree operations is faster than OS interactions with the filesystem directly but relatively slower than the indexes provided by `image.FileCatalog` and `file.Index`.

`filetre.FileTree` objects can be created with a corresponding `file.Index` object by leveraging the `filetree.Builder` object, which aids in the indexing of files. 
