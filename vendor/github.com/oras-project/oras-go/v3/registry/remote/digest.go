/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remote

import (
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// Sources of a digest, named in the error of [checkDigestAlgorithm] so a caller
// can tell an identity the registry chose from one the caller named.
const (
	// digestSourceHeader is a digest the registry reported in the
	// Docker-Content-Digest response header.
	digestSourceHeader = "the Docker-Content-Digest header"
	// digestSourceReference is a digest the caller named in a reference.
	digestSourceReference = "the reference digest"
	// digestSourceDescriptor is a digest the caller passed in a descriptor.
	digestSourceDescriptor = "the descriptor digest"
)

// isSupportedAlgorithm is the oras-go allowlist as seen by this package.
//
// It is a variable only so tests can narrow the allowlist below the set
// go-digest registers. With go-digest v1.0.0 the two sets are identical and
// there is no API to register another algorithm, so narrowing the allowlist is
// the only way to build the "registered but not allowlisted" state that
// [checkDigestAlgorithm] exists to reject.
var isSupportedAlgorithm = descriptor.IsSupportedAlgorithm

// checkDigestAlgorithm reports whether d uses a digest algorithm that
// oras-go acts on. The allowlist in [descriptor.IsSupportedAlgorithm] is
// authoritative even when a dependency registers an algorithm that go-digest
// would otherwise accept, so a digest cannot enter the package in an algorithm
// that later reads or pushes refuse.
//
// source names where d came from. A rejection wraps [errdef.ErrUnsupported] and
// names the algorithm, since the caller cannot tell from a mismatch alone
// whether the registry ignored the requested identity or the algorithm was
// never supported.
//
// A malformed d is not reported here: the parse at the call site classifies it.
// This keeps the check usable before parsing, which is where a non-allowlisted
// algorithm must be rejected rather than reported as a malformed digest.
func checkDigestAlgorithm(source string, d digest.Digest) error {
	s := string(d)
	i := strings.IndexByte(s, ':')
	if i <= 0 || i+1 == len(s) {
		return nil
	}
	alg := digest.Algorithm(s[:i])
	if !isSupportedAlgorithm(alg) || !alg.Available() {
		return fmt.Errorf("%s: unsupported digest algorithm %q: %w", source, alg, errdef.ErrUnsupported)
	}
	return nil
}

// checkReferenceDigestAlgorithm checks the digest of ref, if any, against the
// oras-go allowlist. Reference parsing only asks go-digest whether the
// algorithm is available, so a manifest reference must be checked here before
// a request is issued; a tag-only reference has no digest and passes.
func checkReferenceDigestAlgorithm(ref properties.Reference) error {
	if ref.Digest == "" {
		return nil
	}
	return checkDigestAlgorithm(digestSourceReference, digest.Digest(ref.Digest))
}

// parseSupportedDigest parses value and checks its algorithm against the
// oras-go allowlist, naming source in the error.
//
// The algorithm is checked before the rest of the digest so a non-allowlisted
// algorithm is reported as [errdef.ErrUnsupported] with its source, instead of
// as a malformed digest from go-digest.
func parseSupportedDigest(source, value string) (digest.Digest, error) {
	if err := checkDigestAlgorithm(source, digest.Digest(value)); err != nil {
		return "", err
	}
	d, err := digest.Parse(value)
	if err != nil {
		return "", err
	}
	return d, nil
}
