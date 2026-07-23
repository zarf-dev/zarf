# Go RPM Utils

[![Go Reference](https://pkg.go.dev/badge/github.com/sassoftware/go-rpmutils.svg)](https://pkg.go.dev/github.com/sassoftware/go-rpmutils)

go-rpmutils is a library written in [go](http://golang.org) for parsing and extracting content from [RPMs](http://www.rpm.org).

## Overview

go-rpmutils provides a few interfaces for handling RPM packages. There is a highlevel `Rpm` struct that provides access to the RPM header and [CPIO](https://en.wikipedia.org/wiki/Cpio) payload. The CPIO payload can be extracted to a filesystem location via the `ExpandPayload` function or through a Reader interface, similar to the [tar implementation](https://golang.org/pkg/archive/tar/) in the go standard library.

## Example

```go
// Opening a RPM file
f, err := os.Open("foo.rpm")
if err != nil {
    panic(err)
}
rpm, err := rpmutils.ReadRpm(f)
if err != nil {
    panic(err)
}
// Getting metadata
nevra, err := rpm.Header.GetNEVRA()
if err != nil {
    panic(err)
}
fmt.Println(nevra)
provides, err := rpm.Header.GetStrings(rpmutils.PROVIDENAME)
if err != nil {
    panic(err)
}
fmt.Println("Provides:")
for _, p := range provides {
    fmt.Println(p)
}
// Extracting payload
if err := rpm.ExpandPayload("destdir"); err != nil {
    panic(err)
}
```

## Validating Signatures

rpmutils supports validating PGP signatures embedded in RPM files.

```go
import (
    "github.com/sassoftware/go-rpmutils"
    "github.com/ProtonMail/go-crypto/openpgp"
)

func main() {
    kf, err := os.Open("trusted.pgp")
    keyring, err := openpgp.ReadArmoredKeyRing(kf)
    f, err := os.Open("foo.rpm")
    hdr, sigs, err := rpmutils.Verify(f, keyring)
}
```

Passing `nil` as the keyring will parse the signature without validating it, so
that the signers' key ID can be inspected.

By default rpmutils uses the
[ProtonMail](https://github.com/ProtonMail/go-crypto) PGP implementation, which
supports PGP v4 and later signatures. PGP v4 was released in 1998, and yet some
still-supported Linux distributions contain RPMs with v3 signatures.

Depending on your needs you may want to use the
[pgpkeys-eu](https://github.com/pgpkeys-eu/go-crypto) soft fork, which re-adds
v3 signature support. To consume it, the binary being built must have a
`replace` directive, and must set the `pgp3` tag to enable the related
validation code in rpmutils:

```
go mod edit -replace github.com/ProtonMail/go-crypto=github.com/pgpkeys-eu/go-crypto@main
go build -tags pgp3
```

### Upgrading from versions before v0.4.0

Previous versions of rpmutils used the standard library
`golang.org/x/crypto/openpgp` implementation, which has been deprecated for some
time. Most callers that are verifying or signing RPMs will just need to change
imports to `github.com/ProtonMail/go-crypto/openpgp` .

There are two known regressions with the ProtonMail implementation. The first is
that PGP v3 signatures are no longer supported. If this is important to you,
then see the above note about using the pgpkeys-eu fork instead.

The second is that signing with a HSM-bound private key (`crypto.Signer`) of
type other than RSA is currently not supported by ProtonMail. Hopefully a future
release will restore this functionality.

## Contributing

1. Read contributor agreement
2. Fork it
3. Create your feature branch (`git checkout -b my-new-feature`)
4. Commit your changes (`git commit -a`). Make sure to include a Signed-off-by line per the contributor agreement.
5. Push to the branch (`git push origin my-new-feature`)
6. Create new Pull Request

## License

go-rpmutils is released under the Apache 2.0 license. See [LICENSE](https://github.com/sassoftware/go-rpmutils/blob/master/LICENSE).
