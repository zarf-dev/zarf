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
	"fmt"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/registry/remote/policy"
)

// inPolicyEvaluationKey is a context key set while the policy verifies an
// image, so that a verifier fetching signature artifacts from this
// repository does not evaluate the policy again.
type inPolicyEvaluationKey struct{}

// withInPolicyEvaluation returns a context marked as inside a policy
// evaluation.
func withInPolicyEvaluation(ctx context.Context) context.Context {
	return context.WithValue(ctx, inPolicyEvaluationKey{}, true)
}

// checkPolicy validates repository access before network I/O. It evaluates
// reference-level requirements for every reference and digest-dependent
// requirements when the reference already carries a digest. If no policy is
// configured (Policy is nil), this is a no-op.
//
// Signature requirements (signedBy/sigstoreSigned) are otherwise evaluated
// by checkPolicyResolved when an operation resolves or supplies a manifest
// descriptor. Blob operations and operations without a manifest remain
// reference-level only. Operations that write content or metadata to the
// repository use checkWritePolicy instead, which skips signature
// requirements.
func (r *Repository) checkPolicy(ctx context.Context, reference string) error {
	if ctx.Value(inPolicyEvaluationKey{}) != nil {
		return nil
	}

	pol := r.policy()
	if pol == nil {
		return nil
	}

	imageRef := r.policyImageReference(reference)
	allowed, err := pol.IsReferenceAllowed(ctx, imageRef)
	if err != nil {
		return policyCheckError(imageRef.Reference, err)
	}
	if !allowed {
		return fmt.Errorf("access denied by policy for %s: %w", imageRef.Reference, policy.ErrDenied)
	}
	imageRef.Digest = r.policyDigest(reference)
	if imageRef.Digest != "" {
		allowed, err = pol.IsResolvedImageAllowed(withInPolicyEvaluation(ctx), imageRef)
		if err != nil {
			return policyCheckError(imageRef.Reference, err)
		}
		if !allowed {
			return fmt.Errorf("access denied by policy for %s: %w", imageRef.Reference, policy.ErrDenied)
		}
	}
	return nil
}

// checkWritePolicy validates reference-level policy requirements for
// operations that write content or metadata to the repository (push, tag,
// delete) and for presence probes of push destinations. Signature
// requirements (signedBy/sigstoreSigned) are skipped: a signature binds to
// the digest of the manifest it signs, which cannot exist before the
// manifest is stored, so enforcing them on writes would refuse every push
// of new content. Reference-level requirements such as reject still apply,
// and digest-scoped policy entries still match through the reference.
func (r *Repository) checkWritePolicy(ctx context.Context, reference string) error {
	if ctx.Value(inPolicyEvaluationKey{}) != nil {
		return nil
	}

	pol := r.policy()
	if pol == nil {
		return nil
	}

	imageRef := r.policyImageReference(reference)
	allowed, err := pol.IsReferenceAllowed(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("policy check failed: %w", err)
	}
	if !allowed {
		return fmt.Errorf("access denied by policy for %s: %w", imageRef.Reference, policy.ErrDenied)
	}
	return nil
}

// checkDescriptorWritePolicy is the write-path counterpart of
// checkDescriptorPolicy: it evaluates reference-level requirements only,
// skipping signature requirements. See checkWritePolicy.
func (r *Repository) checkDescriptorWritePolicy(ctx context.Context, desc ocispec.Descriptor) error {
	if isManifest(r.manifestMediaTypes(), desc) {
		return r.checkWritePolicy(ctx, desc.Digest.String())
	}
	return r.checkWritePolicy(ctx, "")
}

// checkPolicyResolved validates the manifest descriptor a reference resolves
// to against digest-dependent policy requirements. The caller's reference is
// kept for signed identity matching while the resolved digest is used for
// signature lookup and payload validation. Callers must first call checkPolicy
// with the same reference because digest references are fully checked there.
func (r *Repository) checkPolicyResolved(ctx context.Context, reference string, desc ocispec.Descriptor) error {
	if ctx.Value(inPolicyEvaluationKey{}) != nil {
		return nil
	}
	// Digest references are fully evaluated by checkPolicy before any
	// registry request is made.
	if r.policyDigest(reference) != "" {
		return nil
	}

	pol := r.policy()
	if pol == nil {
		return nil
	}

	policyReference := reference
	if policyReference == "" {
		policyReference = desc.Digest.String()
	}
	imageRef := r.policyImageReference(policyReference)
	imageRef.Digest = desc.Digest

	// Mark the context as inside a policy evaluation so verifiers that call
	// back into this repository (e.g. to fetch signature artifacts) do not
	// recursively evaluate policy.
	allowed, err := pol.IsResolvedImageAllowed(withInPolicyEvaluation(ctx), imageRef)
	if err != nil {
		return policyCheckError(imageRef.Reference, err)
	}
	if !allowed {
		return fmt.Errorf("access denied by policy for %s: %w", imageRef.Reference, policy.ErrDenied)
	}
	return nil
}

// policyCheckError wraps an error returned by the policy evaluator. A
// requirement that denied the image and said why (an error wrapping
// policy.ErrDenied, such as unsigned content) is reported like any other
// denial of reference, keeping the reason. Anything else is an evaluation
// failure.
func policyCheckError(reference string, err error) error {
	if errors.Is(err, policy.ErrDenied) {
		return fmt.Errorf("access denied by policy for %s: %w", reference, err)
	}
	return fmt.Errorf("policy check failed: %w", err)
}

// checkManifestPolicy evaluates both policy phases for a known manifest.
func (r *Repository) checkManifestPolicy(ctx context.Context, reference string, desc ocispec.Descriptor) error {
	if err := r.checkPolicy(ctx, reference); err != nil {
		return err
	}
	return r.checkPolicyResolved(ctx, reference, desc)
}

// checkDescriptorPolicy evaluates signature requirements only for manifests.
func (r *Repository) checkDescriptorPolicy(ctx context.Context, desc ocispec.Descriptor) error {
	if isManifest(r.manifestMediaTypes(), desc) {
		return r.checkManifestPolicy(ctx, desc.Digest.String(), desc)
	}
	return r.checkPolicy(ctx, "")
}

// policyImageReference builds the policy image reference for the given
// caller reference, falling back to the repository reference when empty.
func (r *Repository) policyImageReference(reference string) policy.ImageReference {
	repoRef := r.Reference()
	ref := repoRef.String()
	if reference != "" {
		if parsed, err := r.ParseReference(reference); err == nil {
			// Policy verifiers historically receive the digest without a tag.
			if parsed.Digest != "" {
				parsed.Tag = ""
			}
			ref = parsed.String()
		} else {
			ref = reference
		}
	}
	return policy.ImageReference{
		Transport: policy.TransportNameDocker,
		Scope:     repoRef.Registry + "/" + repoRef.Repository,
		Reference: ref,
	}
}

// policyDigest returns the digest carried by reference, if any.
func (r *Repository) policyDigest(reference string) digest.Digest {
	if reference == "" {
		return ""
	}
	ref, err := r.ParseReference(reference)
	if err != nil {
		return ""
	}
	d, err := digest.Parse(ref.GetReference())
	if err != nil {
		return ""
	}
	return d
}
