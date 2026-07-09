# restruct [![Build Status](https://travis-ci.org/go-restruct/restruct.svg)](https://travis-ci.org/go-restruct/restruct) [![codecov.io](http://codecov.io/github/go-restruct/restruct/coverage.svg?branch=master)](http://codecov.io/github/go-restruct/restruct?branch=master) [![godoc.org](http://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://godoc.org/github.com/go-restruct/restruct) [![Go Report Card](https://goreportcard.com/badge/github.com/go-restruct/restruct)](https://goreportcard.com/report/github.com/go-restruct/restruct)
`restruct` is a library for reading and writing binary data in Go. Similar to
lunixbochs `struc` and `encoding/binary`, this library reads data based on the
layout of structures and, like `struc`, based on what is contained in struct
tags.

To install Restruct, use the following command:

```
go get github.com/go-restruct/restruct
```

`restruct` aims to provide a clean, flexible, robust implementation of struct
packing. In the future, through fast-path optimizations and code generation, it
also aims to be quick, but it is currently very slow.

`restruct` currently requires Go 1.7+.

## Status

  * As of writing, coverage is hovering around 95%, but more thorough testing
    is always useful and desirable.
  * Unpacking and packing are fully functional.
  * More optimizations are probably possible.

## Example

```go
package main

import (
	"encoding/binary"
	"io/ioutil"
	"os"

	"github.com/go-restruct/restruct"
)

type Record struct {
	Message string `struct:"[128]byte"`
}

type Container struct {
	Version   int `struct:"int32"`
	NumRecord int `struct:"int32,sizeof=Records"`
	Records   []Record
}

func main() {
	var c Container

	file, _ := os.Open("records")
	defer file.Close()
	data, _ := ioutil.ReadAll(file)

	restruct.Unpack(data, binary.LittleEndian, &c)
}
```
