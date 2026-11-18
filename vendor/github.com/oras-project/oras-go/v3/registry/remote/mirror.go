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
	"context"
	"errors"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// PullFromMirror constants control when a mirror should be used.
const (
	// PullFromMirrorAll allows all references (tags and digests).
	PullFromMirrorAll = "all"

	// PullFromMirrorDigestOnly allows only digest references.
	PullFromMirrorDigestOnly = "digest-only"

	// PullFromMirrorTagOnly allows only tag references.
	PullFromMirrorTagOnly = "tag-only"
)

// mirrorRepository pairs a Repository with its pull policy.
type mirrorRepository struct {
	*Repository
	pullFromMirror string
}

// shouldUseForReference reports whether this mirror should handle the given
// reference string based on the PullFromMirror policy.
func (m *mirrorRepository) shouldUseForReference(reference string) bool {
	isDigest := isDigestReference(reference)
	switch m.pullFromMirror {
	case PullFromMirrorDigestOnly:
		return isDigest
	case PullFromMirrorTagOnly:
		return !isDigest
	default:
		// "all" or empty string
		return true
	}
}

// isDigestReference reports whether a reference string is a digest reference.
// A reference is considered a digest reference if it contains "@" (e.g.,
// "repo@sha256:...") or if the part before ":" is a supported OCI digest
// algorithm (e.g., "sha256:abc...").
func isDigestReference(reference string) bool {
	if strings.Contains(reference, "@") {
		return true
	}
	// Bare digest references like "sha256:abc...": check the algorithm prefix.
	// The allowlist alone decides, so reference parsing does not depend on
	// which hash packages the final binary happens to link in. It also
	// rejects host:port patterns (e.g., "localhost:5000"), whose prefix is
	// not an algorithm at all.
	if i := strings.Index(reference, ":"); i > 0 {
		return descriptor.IsSupportedAlgorithm(digest.Algorithm(reference[:i]))
	}
	return false
}

// mirrorReference reduces a possibly fully-qualified reference to a form that
// resolves against a mirror whose registry (and possibly repository) differ
// from the primary's. A mirror Repository validates that a fully-qualified
// reference matches its own base, so the primary's "registry/repo:tag" would be
// rejected as a host mismatch. Stripping it to a bare tag (or "@digest") lets
// the mirror combine it with its own base. Inputs that are already bare are
// returned unchanged.
func mirrorReference(reference string) string {
	// Digest form: keep the leading "@" so the mirror treats it as a digest.
	if i := strings.IndexByte(reference, '@'); i != -1 {
		return reference[i:]
	}
	// Fully-qualified tag reference: keep only the tag component.
	if ref, err := properties.NewReference(reference); err == nil && ref.GetReference() != "" {
		return ref.GetReference()
	}
	// Already a bare tag (or unparseable): use as-is.
	return reference
}

// isMirrorFallbackError reports whether the error should trigger a fallback
// to the next mirror or the primary registry. Context cancellation and
// deadline exceeded errors are not retryable.
func isMirrorFallbackError(err error) bool {
	if err == nil {
		return false
	}
	// errors.Is, not ==: the transport wraps context errors, so a comparison
	// against the sentinels would miss a cancelled context and go on to try
	// every remaining mirror and then the primary.
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// tryMirrors runs the operation against each mirror whose pull policy admits
// reference, in order, and falls back to the primary repository. reference is
// used only for mirror policy matching; it is not what the operations receive.
// A mirror result rejected by accept is discarded and iteration continues, so T
// must not own resources needing release unless accept always returns true. A
// mirror error that isMirrorFallbackError rejects aborts without trying the
// primary.
func tryMirrors[T any](
	mirrors []mirrorRepository,
	primary *Repository,
	reference string,
	mirrorOp func(*Repository) (T, error),
	primaryOp func(*Repository) (T, error),
	accept func(T) bool,
) (T, error) {
	var zero T

	for i := range mirrors {
		if !mirrors[i].shouldUseForReference(reference) {
			continue
		}

		result, err := mirrorOp(mirrors[i].Repository)
		if err == nil {
			if accept(result) {
				return result, nil
			}
			continue
		}

		if !isMirrorFallbackError(err) {
			return zero, err
		}
	}

	return primaryOp(primary)
}

// tryMirrorsRef is tryMirrors for an operation that takes a reference. Mirrors
// receive the reference reduced by mirrorReference, and the primary receives it
// unchanged.
func tryMirrorsRef[T any](
	mirrors []mirrorRepository,
	primary *Repository,
	reference string,
	op func(*Repository, string) (T, error),
) (T, error) {
	return tryMirrors(
		mirrors,
		primary,
		reference,
		func(repo *Repository) (T, error) { return op(repo, mirrorReference(reference)) },
		func(repo *Repository) (T, error) { return op(repo, reference) },
		acceptAll[T],
	)
}

// acceptAll is a tryMirrors accept function that accepts every result.
func acceptAll[T any](T) bool { return true }
