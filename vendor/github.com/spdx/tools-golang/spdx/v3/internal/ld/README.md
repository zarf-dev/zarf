# JSON-LD utilities

This package contains JSON-LD utilities shared across the SPDX 3 data models, including  a JSON-LD reader & graph writer
using the official JSON-LD go language implementation: `github.com/piprate/go-ld`, which is able to map to and from
go object models. Subpackage `shaclgen` contains a generator for go source code from SHACL, which has only implemented
the functionality to support SPDX 3.
