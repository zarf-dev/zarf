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

package oras

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/internal/cas"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/internal/status"
	"github.com/oras-project/oras-go/v3/internal/syncutil"
	"golang.org/x/sync/semaphore"
)

// DefaultVerifyOptions provides the default VerifyOptions.
// It is the zero value, provided for convenience. Do not modify it: the
// change would apply to every caller in the process that passes it.
var DefaultVerifyOptions VerifyOptions = VerifyOptions{
	VerifyGraphOptions: DefaultVerifyGraphOptions,
}

// VerifyOptions contains parameters for [oras.Verify].
type VerifyOptions struct {
	VerifyGraphOptions
}

// DefaultVerifyGraphOptions provides the default VerifyGraphOptions.
// It is the zero value, provided for convenience. Do not modify it: the
// change would apply to every caller in the process that passes it.
var DefaultVerifyGraphOptions VerifyGraphOptions

// VerifyGraphOptions contains parameters for [oras.VerifyGraph].
type VerifyGraphOptions struct {
	// Concurrency limits the maximum number of concurrent verify tasks.
	// If less than or equal to 0, a default (currently 8) is used.
	Concurrency int
	// MaxMetadataBytes limits the maximum size of the metadata (i.e.
	// manifests) that can be cached in memory while walking the graph.
	// If less than or equal to 0, a default (currently 4 MiB) is used.
	MaxMetadataBytes int64
	// PreVerify handles the current descriptor before it is verified.
	// PreVerify can return SkipNode to signal that desc, and the sub-DAG
	// rooted at desc, should be recorded as skipped instead of verified.
	PreVerify func(ctx context.Context, desc ocispec.Descriptor) error
	// PostVerify handles the current descriptor after an attempt to verify
	// it. verifyErr is the error encountered while verifying desc, if any;
	// it is nil when desc passed verification. Returning a non-nil error
	// from PostVerify aborts the walk, unlike a verification failure
	// itself, which does not.
	PostVerify func(ctx context.Context, desc ocispec.Descriptor, verifyErr error) error
	// OnVerifySkipped will be called when desc is skipped because its
	// digest was already present in KnownVerified.
	OnVerifySkipped func(ctx context.Context, desc ocispec.Descriptor) error
	// FindSuccessors finds the successors of the current node.
	// A node for which FindSuccessors returns successors is not itself read
	// in full by the verifier, so a custom FindSuccessors must read and
	// verify such nodes itself. Foreign (non-distributable) layers are
	// dropped from the successors and appear in neither Skipped nor Failed.
	// fetcher provides cached access to the source storage, and is suitable
	// for fetching non-leaf nodes like manifests. Since anything fetched from
	// fetcher will be cached in the memory, it is recommended to use the
	// original source storage to fetch large blobs.
	// If FindSuccessors is nil, content.Successors will be used.
	FindSuccessors func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) ([]ocispec.Descriptor, error)
	// KnownVerified is an optional, caller-provided record of digests that are
	// already known to be intact. Leaf blobs whose digest is present in
	// KnownVerified are not re-fetched; they are recorded as skipped in the
	// report instead. Manifests are always re-read, since that is the only
	// way to discover their successors. On success, VerifyGraph adds every
	// digest it verifies to KnownVerified, so passing the same VerifiedSet
	// across multiple Verify/VerifyGraph calls avoids re-verifying blobs
	// shared between them. KnownVerified is nil by default, so every node is
	// freshly verified.
	KnownVerified *VerifiedSet
}

// VerifiedSet records digests that have already passed content integrity
// verification. It is safe for concurrent use, and a single VerifiedSet may
// be shared and reused across multiple Verify/VerifyGraph calls that read
// from the same underlying storage. Do not share a VerifiedSet between
// different storages; see VerifyGraphOptions.KnownVerified.
//
// Digests added to a VerifiedSet are trusted without being re-read, so only
// add digests that were actually verified.
type VerifiedSet struct {
	lock  sync.RWMutex
	sizes map[digest.Digest]int64
}

// NewVerifiedSet returns an empty VerifiedSet ready for use.
func NewVerifiedSet() *VerifiedSet {
	return &VerifiedSet{sizes: make(map[digest.Digest]int64)}
}

// Contains reports whether desc has already been recorded as verified with
// the same digest and size.
func (s *VerifiedSet) Contains(desc ocispec.Descriptor) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	size, ok := s.sizes[desc.Digest]
	return ok && size == desc.Size
}

// Add records desc as verified.
func (s *VerifiedSet) Add(desc ocispec.Descriptor) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.sizes == nil {
		s.sizes = make(map[digest.Digest]int64)
	}
	s.sizes[desc.Digest] = desc.Size
}

// Len returns the number of digests currently recorded.
func (s *VerifiedSet) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.sizes)
}

// FailedNode describes a descriptor that failed content integrity
// verification, together with the error encountered.
type FailedNode struct {
	Descriptor ocispec.Descriptor
	Err        error
}

// VerifyGraphReport summarizes the outcome of a VerifyGraph call.
//
// VerifyGraph runs in "on error resume next" mode: a node that fails
// verification does not abort the walk of the rest of the graph, so a report
// can describe a graph that is partially intact. When a manifest cannot be
// read, its successors cannot be discovered either, so the sub-DAG rooted at
// that manifest is reachable only up to that point; Failed reports the
// manifest itself, and nodes beneath it that happen to be reachable by some
// other path in the graph are still verified independently.
type VerifyGraphReport struct {
	// Verified lists the descriptors that were fetched in full and passed
	// content integrity verification during this call.
	Verified []ocispec.Descriptor
	// Skipped lists the descriptors that were not re-verified.
	Skipped []ocispec.Descriptor
	// Failed lists the descriptors that failed content integrity
	// verification, together with the error encountered for each.
	Failed []FailedNode
}

// OK reports whether verification recorded any failures.
// Skipped descriptors do not make the report unsuccessful.
func (r *VerifyGraphReport) OK() bool {
	return r != nil && len(r.Failed) == 0
}

// verifyGraphReport is the mutable, concurrency-safe accumulator used during
// graph verification. report returns a snapshot as the public report type.
type verifyGraphReport struct {
	mu       sync.Mutex
	verified []ocispec.Descriptor
	skipped  []ocispec.Descriptor
	failed   []FailedNode
}

func newVerifyGraphReport() *verifyGraphReport {
	return &verifyGraphReport{}
}

// report returns a snapshot of the accumulated results.
func (r *verifyGraphReport) report() *VerifyGraphReport {
	r.mu.Lock()
	defer r.mu.Unlock()

	return &VerifyGraphReport{
		Verified: append([]ocispec.Descriptor(nil), r.verified...),
		Skipped:  append([]ocispec.Descriptor(nil), r.skipped...),
		Failed:   append([]FailedNode(nil), r.failed...),
	}
}

func (r *verifyGraphReport) addVerified(desc ocispec.Descriptor) {
	r.mu.Lock()
	r.verified = append(r.verified, desc)
	r.mu.Unlock()
}

func (r *verifyGraphReport) addSkipped(desc ocispec.Descriptor) {
	r.mu.Lock()
	r.skipped = append(r.skipped, desc)
	r.mu.Unlock()
}

func (r *verifyGraphReport) addFailed(desc ocispec.Descriptor, err error) {
	r.mu.Lock()
	r.failed = append(r.failed, FailedNode{Descriptor: desc, Err: err})
	r.mu.Unlock()
}

// merge adds the results from a completed verification into this accumulator.
func (r *verifyGraphReport) merge(other *VerifyGraphReport) {
	if other == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.verified = append(r.verified, other.Verified...)
	r.skipped = append(r.skipped, other.Skipped...)
	r.failed = append(r.failed, other.Failed...)
}

// VerifyReport is the result of Verify. It embeds the VerifyGraphReport for
// the DAG rooted at Node.
type VerifyReport struct {
	// Node is the descriptor that ref resolved to.
	Node ocispec.Descriptor
	*VerifyGraphReport
}

// OK reports whether verification recorded any failures.
// It returns false for a nil report.
func (r *VerifyReport) OK() bool {
	return r != nil && r.VerifyGraphReport.OK()
}

// Verify resolves ref against src and verifies the content integrity of the
// rooted directed acyclic graph (DAG) identified by the resolved descriptor,
// such as an artifact and everything it references.
//
// Verify walks the whole graph and discards the content it reads rather than
// writing it anywhere; unlike Copy, a failure to verify one node does not stop
// the walk of the rest of the graph ("on error resume next"), so a caller can
// learn about every corrupt or missing node in one call.
//
// Inspect the returned VerifyReport, or call its OK method, to find out
// whether verification fully succeeded. If src implements [registry.ReferenceFetcher],
// the root manifest is read while resolving ref, so a corrupt or unreadable root
// is returned as an error with a nil report rather than recorded as a failed node.
//
// Returns the descriptor of the root node, plus a report, on completion. A
// non-nil error is only returned for failures that stop the walk itself, such
// as failing to resolve ref or a context cancellation; per-node verification
// failures are recorded in the report, not returned as an error. If the walk
// stops after ref was resolved, the report is still returned and describes
// the nodes processed up to that point; if ref cannot be resolved, the report
// is nil.
func Verify(ctx context.Context, src ReadOnlyTarget, ref string, opts VerifyOptions) (ocispec.Descriptor, *VerifyReport, error) {
	if src == nil {
		return ocispec.Descriptor{}, nil, newCopyError("Verify", CopyErrorOriginSource, ocispec.Descriptor{}, errors.New("nil source target"))
	}

	// use caching proxy on non-leaf nodes
	if opts.MaxMetadataBytes <= 0 {
		opts.MaxMetadataBytes = defaultCopyMaxMetadataBytes
	}
	proxy := cas.NewProxyWithLimit(src, cas.NewMemory(), opts.MaxMetadataBytes)
	root, err := resolveRoot(ctx, src, ref, proxy)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}

	graphReport, err := verifyGraph(ctx, src, root, proxy, nil, nil, opts.VerifyGraphOptions)

	// report holds whatever was verified before the walk stopped, so it
	// is returned along with the error.
	return root, &VerifyReport{Node: root, VerifyGraphReport: graphReport}, err
}

// VerifyGraph verifies the content integrity of a rooted directed acyclic
// graph (DAG), such as an artifact, in the source CAS. The root node (e.g. a
// manifest of the artifact) is identified by a descriptor.
//
// See the Verify documentation for the "on error resume next" behavior and
// how to interpret the returned report.
func VerifyGraph(ctx context.Context, src content.ReadOnlyStorage, root ocispec.Descriptor, opts VerifyGraphOptions) (*VerifyGraphReport, error) {
	if src == nil {
		return nil, newCopyError("VerifyGraph", CopyErrorOriginSource, ocispec.Descriptor{}, errors.New("nil source target"))
	}

	return verifyGraph(ctx, src, root, nil, nil, nil, opts)
}

// verifyGraph does the actual work of VerifyGraph, with specified caching,
// concurrency limiter and tracker, mirroring copyGraph's structure. proxy,
// limiter and tracker may each be nil, in which case a fresh one is created;
// callers that already have a proxy warmed up by resolveRoot (i.e. Verify)
// pass it through so the root node it cached is not re-fetched.
func verifyGraph(ctx context.Context, src content.ReadOnlyStorage, root ocispec.Descriptor,
	proxy *cas.Proxy, limiter *semaphore.Weighted, tracker *status.Tracker, opts VerifyGraphOptions) (*VerifyGraphReport, error) {
	if proxy == nil {
		// use a caching proxy on non-leaf nodes, exactly as copyGraph does,
		// so that a manifest fetched to discover its successors is not
		// fetched a second time.
		if opts.MaxMetadataBytes <= 0 {
			opts.MaxMetadataBytes = defaultCopyMaxMetadataBytes
		}
		proxy = cas.NewProxyWithLimit(src, cas.NewMemory(), opts.MaxMetadataBytes)
	}
	if limiter == nil {
		if opts.Concurrency <= 0 {
			opts.Concurrency = defaultConcurrency
		}
		limiter = semaphore.NewWeighted(int64(opts.Concurrency))
	}
	if tracker == nil {
		tracker = status.NewTracker()
	}
	report := newVerifyGraphReport()

	if opts.FindSuccessors == nil {
		opts.FindSuccessors = content.Successors
	}

	var fn syncutil.GoFunc[ocispec.Descriptor]
	fn = func(ctx context.Context, region *syncutil.LimitedRegion, desc ocispec.Descriptor) (err error) {
		// skip the descriptor if another goroutine is working on it
		done, committed := tracker.TryCommit(desc)
		if !committed {
			return nil
		}
		defer func() {
			if err == nil {
				close(done)
			}
		}()

		// Skip content already verified by a previous call sharing this
		// VerifiedSet. This only applies to leaf nodes (blobs): a manifest
		// must still be fetched and parsed on every call regardless of
		// whether its own bytes were verified before, since that is the
		// only way to discover which children to verify this time, and a
		// manifest is metadata-sized (bounded by MaxMetadataBytes) so
		// re-reading it is cheap.
		isManifest := descriptor.IsManifest(desc)
		if !isManifest && opts.KnownVerified != nil && opts.KnownVerified.Contains(desc) {
			if opts.OnVerifySkipped != nil {
				if err := opts.OnVerifySkipped(ctx, desc); err != nil {
					return err
				}
			}
			report.addSkipped(desc)
			return nil
		}

		if opts.PreVerify != nil {
			if err := opts.PreVerify(ctx, desc); err != nil {
				if err == SkipNode {
					report.addSkipped(desc)
					return nil
				}
				return err
			}
		}

		successors, verifyErr := opts.FindSuccessors(ctx, proxy, desc)
		if verifyErr != nil {
			verifyErr = fmt.Errorf("failed to read %s: %w", desc.Digest, verifyErr)
		} else if len(successors) == 0 {
			if err := content.Verify(ctx, src, desc); err != nil {
				verifyErr = fmt.Errorf("failed to read %s: %w", desc.Digest, err)
			}
		}

		if verifyErr == nil {
			successors = removeForeignLayers(successors)
			for _, successor := range successors {
				if err := validateSuccessor(successor); err != nil {
					verifyErr = fmt.Errorf("invalid successor descriptor: successor media type: %s; successor size: %d; successor digest: %s: %w",
						successor.MediaType, successor.Size, successor.Digest, err)
					break
				}
			}
		}

		// A cancelled or expired context is not a verification result:
		// stop the walk immediately.
		if verifyErr != nil && ctx.Err() != nil {
			return ctx.Err()
		}

		// record the outcome before PostVerify, so that a PostVerify error
		// aborting the walk does not lose the node from the report.
		if verifyErr != nil {
			report.addFailed(desc, verifyErr)
		} else {
			report.addVerified(desc)
			if opts.KnownVerified != nil {
				opts.KnownVerified.Add(desc)
			}
		}

		if opts.PostVerify != nil {
			if err := opts.PostVerify(ctx, desc, verifyErr); err != nil {
				return err
			}
		}

		if verifyErr != nil {
			// on error resume next: this sub-DAG cannot be reliably
			// discovered further, since desc itself failed to read or
			// verify, but the walk continues for the rest of the graph.
			return nil
		}

		if len(successors) == 0 {
			return nil
		}

		// process successors and wait for them to complete
		region.End()
		if err := syncutil.Go(ctx, limiter, fn, successors...); err != nil {
			return err
		}
		for _, node := range successors {
			done, committed := tracker.TryCommit(node)
			if committed {
				return fmt.Errorf("%s: %s: successor not committed", desc.Digest, node.Digest)
			}
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return region.Start()
	}

	if err := syncutil.Go(ctx, limiter, fn, root); err != nil {
		return report.report(), newCopyError("VerifyGraph", CopyErrorOriginSource, root, err)
	}
	return report.report(), nil
}
