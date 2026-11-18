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

package oci

import (
	"context"
	_ "crypto/sha256" // SHA-256 path-digest verification
	_ "crypto/sha512" // SHA-384 and SHA-512 path-digest verification
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/internal/syncutil"
	"golang.org/x/sync/semaphore"
)

// defaultVerifyConcurrency is the default value of VerifyOptions.Concurrency.
// OCI verification hashes local blob files from disk rather than performing
// network round trips, so it intentionally does not follow the graph
// copy/verify default.
const defaultVerifyConcurrency = 3

// VerifyOptions contains parameters for [Verify].
type VerifyOptions struct {
	// Concurrency limits the maximum number of concurrent blob verifications.
	// If less than or equal to 0, a default (currently 3) is used.
	Concurrency int
}

// VerifyReport summarizes the outcome of [Verify].
//
// Verify runs in "on error resume next" mode: a blob that fails verification
// does not abort the sweep of the rest of the layout, so a report can describe
// a layout that is partially intact.
type VerifyReport struct {
	// Verified lists blob paths whose content hashes to the digest named by
	// the path.
	Verified []VerifyEntry
	// Failed lists blob paths whose content does not match the path digest, or
	// that cannot be read.
	Failed []VerifyEntry
	// Unverifiable lists paths that cannot be verified because the digest
	// algorithm is unsupported, the file name is not a valid encoded digest,
	// or the entry is not a blob file. These are the paths [Store.GC] would
	// skip.
	//
	// It also lists what stops reachability from being followed: a manifest or
	// index that is present but cannot be read or parsed (Path is its blob
	// path), and a descriptor with a malformed digest (Path is the referrer, or
	// index.json). The Reason says why. Such a blob may also be in Verified or
	// Failed. Unreferenced is left empty then, because content that only that
	// manifest references would look unreferenced.
	Unverifiable []VerifyEntry
	// Layout lists problems with the structure of the layout outside of
	// blobs/: a missing or invalid oci-layout or index.json, and the
	// leftovers of interrupted writes (files in ingest/ and temporary
	// index.json or oci-layout files in the layout root). Nothing is deleted,
	// so no leftover expiry applies; each leftover carries its modification
	// time instead.
	//
	// Layout findings do not indicate corrupt content, and a layout that once
	// had a write killed is not corrupt, so they are kept apart from Failed.
	// A layout problem never stops the verification of blobs.
	Layout []LayoutFinding

	// Missing lists blob paths that index.json references, directly or
	// through manifests and indexes, but that are absent from blobs/. The
	// Reason wraps [errdef.ErrNotFound] and names the referrer. A missing blob
	// is reported here only: it is not also in Failed. A subject that is
	// absent is not reported, since a referrer may legitimately outlive it.
	Missing []VerifyEntry
	// Unreferenced lists blobs that are present and verified but not
	// reachable, by the rule [Store.GC] applies: tagged manifests and what
	// they reference, plus untagged referrers whose subject chain reaches a
	// tagged manifest. An untagged manifest that is not such a referrer, and
	// everything only it references, is unreferenced. This is exactly what GC
	// would remove, among the blobs that verified. Reason is nil.
	//
	// A blob that fails verification is in Failed even if nothing references
	// it, never here. Unreferenced is empty when reachability could not be
	// followed completely: see Unverifiable.
	//
	// Unreferenced is only authoritative on a quiescent layout. Verify takes
	// no lock, applies no grace period, and does not coordinate with writers:
	// a blob that a concurrent [Storage.Push] has already renamed into blobs/
	// but that index.json does not name yet is present and intact, and is
	// reported here even while [Store.GC] would still spare it as recent. Do
	// not delete based on this list without excluding concurrent writers, for
	// example by running against a layout no other process has open.
	Unreferenced []VerifyEntry
}

// VerifyEntry is the result of inspecting a single path under blobs/.
type VerifyEntry struct {
	// Path is the path of the entry relative to the root of fsys, using slash
	// separators as in [io/fs].
	Path string
	// Reason is the error that caused the entry to be classified as Failed,
	// Unverifiable or Missing. It is nil for Verified and Unreferenced entries.
	Reason error
}

type hashItem struct {
	path string
	dgst digest.Digest
}

type verifyAccumulator struct {
	mu           sync.Mutex
	verified     []VerifyEntry
	failed       []VerifyEntry
	unverifiable []VerifyEntry
	layout       []LayoutFinding
	missing      []VerifyEntry
	unreferenced []VerifyEntry
}

func (a *verifyAccumulator) addVerified(p string) {
	a.mu.Lock()
	a.verified = append(a.verified, VerifyEntry{Path: p})
	a.mu.Unlock()
}

func (a *verifyAccumulator) addFailed(p string, reason error) {
	a.mu.Lock()
	a.failed = append(a.failed, VerifyEntry{Path: p, Reason: reason})
	a.mu.Unlock()
}

func (a *verifyAccumulator) addUnverifiable(p string, reason error) {
	a.mu.Lock()
	a.unverifiable = append(a.unverifiable, VerifyEntry{Path: p, Reason: reason})
	a.mu.Unlock()
}

func (a *verifyAccumulator) addLayout(f LayoutFinding) {
	a.mu.Lock()
	a.layout = append(a.layout, f)
	a.mu.Unlock()
}

func (a *verifyAccumulator) addMissing(p string, reason error) {
	a.mu.Lock()
	a.missing = append(a.missing, VerifyEntry{Path: p, Reason: reason})
	a.mu.Unlock()
}

func (a *verifyAccumulator) addUnreferenced(p string) {
	a.mu.Lock()
	a.unreferenced = append(a.unreferenced, VerifyEntry{Path: p})
	a.mu.Unlock()
}

func (a *verifyAccumulator) report() *VerifyReport {
	a.mu.Lock()
	defer a.mu.Unlock()
	return &VerifyReport{
		Verified:     append([]VerifyEntry(nil), a.verified...),
		Failed:       append([]VerifyEntry(nil), a.failed...),
		Unverifiable: append([]VerifyEntry(nil), a.unverifiable...),
		Layout:       append([]LayoutFinding(nil), a.layout...),
		Missing:      append([]VerifyEntry(nil), a.missing...),
		Unreferenced: append([]VerifyEntry(nil), a.unreferenced...),
	}
}

// Verify walks the blobs directory of an OCI image layout in fsys and checks
// that each blob's content matches the digest named by its path.
//
// It also reports problems with the structure of the layout itself in
// [VerifyReport.Layout], and checks reachability from index.json: blobs that
// are referenced but absent are [VerifyReport.Missing], and blobs that are
// present and intact but that [Store.GC] would remove are
// [VerifyReport.Unreferenced]. If index.json cannot be read, reachability is
// not computed and both are empty; the problem is in [VerifyReport.Layout].
//
// Verify takes no lock: it reads index.json and then walks blobs/, so Missing
// and Unreferenced can reflect writes, deletes, or garbage collection that
// happen in between. In particular, a blob from a [Storage.Push] in progress
// may be present before index.json references it, and is then reported as
// unreferenced. Unreferenced is authoritative only when nothing else is
// writing to the layout; see [VerifyReport.Unreferenced].
//
// Verify is read-only: it does not create, modify, or remove any file in fsys.
//
// A blob that fails verification, or a problem with the layout files, does not
// stop the sweep. The returned error is reserved for context cancellation and
// for a layout root or blobs directory that cannot be read. On cancellation after the walk has started, the report describes
// the entries processed before the sweep stopped.
func Verify(ctx context.Context, fsys fs.FS, opts VerifyOptions) (*VerifyReport, error) {
	if err := isContextDone(ctx); err != nil {
		return nil, err
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = defaultVerifyConcurrency
	}

	acc := &verifyAccumulator{}
	index, err := verifyLayout(ctx, fsys, acc)
	if err != nil {
		return nil, err
	}

	algDirs, err := fs.ReadDir(fsys, ocispec.ImageBlobsDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ocispec.ImageBlobsDir, err)
	}

	// present holds the path of every entry the sweep found in a digest
	// directory, whatever its kind, and unlisted the algorithm directories
	// that could not be listed. Missing is computed against them.
	present := make(map[string]struct{})
	unlisted := make(map[string]error)

	var toHash []hashItem
	for _, algDir := range algDirs {
		if err := isContextDone(ctx); err != nil {
			return acc.report(), err
		}
		algPath := path.Join(ocispec.ImageBlobsDir, algDir.Name())
		if !algDir.IsDir() {
			acc.addUnverifiable(algPath, fmt.Errorf("expected a digest algorithm directory"))
			continue
		}

		alg := digest.Algorithm(algDir.Name())
		supported := descriptor.IsSupportedAlgorithm(alg)
		digestEntries, err := fs.ReadDir(fsys, algPath)
		if err != nil {
			readErr := fmt.Errorf("read directory: %w", err)
			acc.addFailed(algPath, readErr)
			unlisted[string(alg)] = readErr
			continue
		}
		for _, digestEntry := range digestEntries {
			entryPath := path.Join(algPath, digestEntry.Name())
			present[entryPath] = struct{}{}
			if !supported {
				acc.addUnverifiable(entryPath, fmt.Errorf("digest algorithm %q: %w", alg, errdef.ErrUnsupported))
				continue
			}
			if !digestEntry.Type().IsRegular() {
				acc.addUnverifiable(entryPath, fmt.Errorf("expected a regular blob file, found %s", digestEntry.Type()))
				continue
			}
			blobDigest := digest.NewDigestFromEncoded(alg, digestEntry.Name())
			if err := blobDigest.Validate(); err != nil {
				acc.addUnverifiable(entryPath, fmt.Errorf("invalid digest %s: %w", blobDigest, err))
				continue
			}
			toHash = append(toHash, hashItem{path: entryPath, dgst: blobDigest})
		}
	}

	if len(toHash) > 0 {
		limiter := semaphore.NewWeighted(int64(concurrency))
		err = syncutil.Go(ctx, limiter, func(ctx context.Context, _ *syncutil.LimitedRegion, item hashItem) error {
			if err := verifyBlob(ctx, fsys, item.path, item.dgst); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				acc.addFailed(item.path, err)
				return nil
			}
			acc.addVerified(item.path)
			return nil
		}, toHash...)
		if err != nil {
			return acc.report(), err
		}
	}

	if index != nil {
		if err := verifyReachability(ctx, fsys, index, present, unlisted, acc); err != nil {
			return acc.report(), err
		}
	}
	return acc.report(), nil
}

func verifyBlob(ctx context.Context, fsys fs.FS, p string, dgst digest.Digest) error {
	f, err := fsys.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()

	verifier := dgst.Verifier()
	buf := make([]byte, 32*1024)
	for {
		if err := isContextDone(ctx); err != nil {
			return err
		}
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := verifier.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	if !verifier.Verified() {
		return fmt.Errorf("%w: %s", content.ErrMismatchedDigest, dgst)
	}
	return nil
}
