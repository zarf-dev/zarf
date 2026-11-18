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

package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/oras-project/oras-go/v3/errdef"
)

// ImageReference represents a reference to an image
type ImageReference struct {
	// Transport is the transport type (e.g., "docker")
	Transport TransportName
	// Scope is the scope within the transport (e.g., "docker.io/library/nginx")
	Scope string
	// Reference is the full reference (e.g., "docker.io/library/nginx:latest")
	Reference string
	// Digest is the manifest digest the reference resolves to, if known.
	// Signature-based requirements (signedBy, sigstoreSigned) verify
	// signatures against the manifest digest, so callers that resolve a tag
	// reference should set it before evaluating those requirements.
	// When empty, verifiers fall back to extracting a digest from Reference.
	Digest digest.Digest
}

// policyScope returns the scope used to look up policy requirements. When the
// reference carries a tag or digest, that fully qualified form is used, so a
// containers-policy.json entry naming an exact tag or digest can match. Scope
// itself stays the repository, which signature verifiers use as the store key.
func (r ImageReference) policyScope() string {
	if r.Scope == "" || r.Reference == "" {
		return r.Scope
	}
	if rest, ok := strings.CutPrefix(r.Reference, r.Scope); ok &&
		len(rest) > 1 && (rest[0] == ':' || rest[0] == '@') {
		return r.Reference
	}
	return r.Scope
}

// SignedByVerifier verifies GPG/simple signing signatures.
// Implementations should verify that the image is signed with a valid key
// as specified in the PRSignedBy requirement.
type SignedByVerifier interface {
	Verify(ctx context.Context, req *PRSignedBy, image ImageReference) (bool, error)
}

// SigstoreVerifier verifies sigstore signatures.
// Implementations should verify that the image is signed with valid sigstore
// signatures as specified in the PRSigstoreSigned requirement.
type SigstoreVerifier interface {
	Verify(ctx context.Context, req *PRSigstoreSigned, image ImageReference) (bool, error)
}

// Evaluator evaluates policy requirements against image references
type Evaluator struct {
	policy           *Policy
	signedByVerifier SignedByVerifier
	sigstoreVerifier SigstoreVerifier
}

// EvaluatorOption configures an Evaluator
type EvaluatorOption func(*Evaluator)

// WithSignedByVerifier sets the verifier for PRSignedBy requirements.
// If not set, evaluating PRSignedBy requirements will return ErrUnsupported.
func WithSignedByVerifier(v SignedByVerifier) EvaluatorOption {
	return func(e *Evaluator) {
		e.signedByVerifier = v
	}
}

// WithSigstoreVerifier sets the verifier for PRSigstoreSigned requirements.
// If not set, evaluating PRSigstoreSigned requirements will return ErrUnsupported.
func WithSigstoreVerifier(v SigstoreVerifier) EvaluatorOption {
	return func(e *Evaluator) {
		e.sigstoreVerifier = v
	}
}

// NewEvaluator creates a new policy evaluator
func NewEvaluator(policy *Policy, opts ...EvaluatorOption) (*Evaluator, error) {
	if policy == nil {
		return nil, fmt.Errorf("policy cannot be nil: %w", errdef.ErrMissingReference)
	}

	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	e := &Evaluator{
		policy: policy,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e, nil
}

// IsImageAllowed determines if an image is allowed by the policy
func (e *Evaluator) IsImageAllowed(ctx context.Context, image ImageReference) (bool, error) {
	return e.isImageAllowed(ctx, image, nil)
}

// isImageAllowed evaluates policy requirements accepted by filter. A nil
// filter evaluates every requirement.
func (e *Evaluator) isImageAllowed(ctx context.Context, image ImageReference, filter func(PolicyRequirement) bool) (bool, error) {
	reqs := e.policy.GetRequirementsForImage(image.Transport, image.policyScope())

	if len(reqs) == 0 {
		// No requirements: treat as a policy error and reject by default for safety.
		return false, fmt.Errorf("no policy requirements found for %s:%s", image.Transport, image.Scope)
	}

	// All requirements must be satisfied
	for _, req := range reqs {
		if filter != nil && !filter(req) {
			continue
		}
		allowed, err := e.evaluateRequirement(ctx, req, image)
		if err != nil {
			if !allowed && errors.Is(err, ErrDenied) {
				// The requirement denied the image and gave a reason.
				return false, fmt.Errorf("%s requirement not satisfied: %w", req.Type(), err)
			}
			return false, fmt.Errorf("failed to evaluate requirement %s: %w", req.Type(), err)
		}
		if !allowed {
			return false, nil
		}
	}

	return true, nil
}

// IsReferenceAllowed determines if an image is allowed by the requirements
// that can be decided from the reference alone, such as reject and
// insecureAcceptAnything. Signature-based requirements (signedBy,
// sigstoreSigned) are skipped: they verify signatures against a manifest
// digest, which a tag reference or a reference-less operation does not
// carry. Callers should evaluate them with IsResolvedImageAllowed once the
// reference is resolved, setting ImageReference.Digest to the resolved
// manifest digest.
func (e *Evaluator) IsReferenceAllowed(ctx context.Context, image ImageReference) (bool, error) {
	return e.isImageAllowed(ctx, image, func(req PolicyRequirement) bool {
		return !requiresResolvedImage(req)
	})
}

// IsResolvedImageAllowed determines if an image is allowed by requirements
// that need its resolved manifest digest, such as signedBy and
// sigstoreSigned. Reference-level requirements are skipped because callers
// should evaluate them first with IsReferenceAllowed.
func (e *Evaluator) IsResolvedImageAllowed(ctx context.Context, image ImageReference) (bool, error) {
	return e.isImageAllowed(ctx, image, requiresResolvedImage)
}

// requiresResolvedImage reports whether the requirement needs the resolved
// manifest digest of the image to be evaluated.
func requiresResolvedImage(req PolicyRequirement) bool {
	switch req.(type) {
	case *PRSignedBy, *PRSigstoreSigned:
		return true
	}
	return false
}

// evaluateRequirement evaluates a single policy requirement
func (e *Evaluator) evaluateRequirement(ctx context.Context, req PolicyRequirement, image ImageReference) (bool, error) {
	switch r := req.(type) {
	case *InsecureAcceptAnything:
		return e.evaluateInsecureAcceptAnything(ctx, r, image)
	case *Reject:
		return e.evaluateReject(ctx, r, image)
	case *PRSignedBy:
		return e.evaluateSignedBy(ctx, r, image)
	case *PRSigstoreSigned:
		return e.evaluateSigstoreSigned(ctx, r, image)
	default:
		return false, fmt.Errorf("unknown requirement type: %T", req)
	}
}

// evaluateInsecureAcceptAnything always accepts the image
func (e *Evaluator) evaluateInsecureAcceptAnything(ctx context.Context, req *InsecureAcceptAnything, image ImageReference) (bool, error) {
	return true, nil
}

// evaluateReject always rejects the image
func (e *Evaluator) evaluateReject(ctx context.Context, req *Reject, image ImageReference) (bool, error) {
	return false, nil
}

// evaluateSignedBy evaluates a signedBy requirement
func (e *Evaluator) evaluateSignedBy(ctx context.Context, req *PRSignedBy, image ImageReference) (bool, error) {
	if e.signedByVerifier == nil {
		return false, fmt.Errorf("signedBy verification requires a SignedByVerifier: %w", errdef.ErrUnsupported)
	}
	return e.signedByVerifier.Verify(ctx, req, image)
}

// evaluateSigstoreSigned evaluates a sigstoreSigned requirement
func (e *Evaluator) evaluateSigstoreSigned(ctx context.Context, req *PRSigstoreSigned, image ImageReference) (bool, error) {
	if e.sigstoreVerifier == nil {
		return false, fmt.Errorf("sigstoreSigned verification requires a SigstoreVerifier: %w", errdef.ErrUnsupported)
	}
	return e.sigstoreVerifier.Verify(ctx, req, image)
}

// ShouldAcceptImage is a convenience function that returns true if the image is allowed
func ShouldAcceptImage(ctx context.Context, policy *Policy, image ImageReference) (bool, error) {
	evaluator, err := NewEvaluator(policy)
	if err != nil {
		return false, err
	}

	return evaluator.IsImageAllowed(ctx, image)
}
