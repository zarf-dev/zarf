package image

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	async "github.com/anchore/go-sync"
	"github.com/anchore/stereoscope/internal/bus"
	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/event"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
)

// Image represents a container image.
type Image struct {
	// image is the raw image metadata and content provider from the GCR lib
	image v1.Image
	// tmpDirGen is a dir generator used by Providers. Multiple directories may
	// be created and cleanup must use this to prevent polluting the disk
	tmpDirGen *file.TempDirGenerator
	// contentCacheDir is where all layer tar cache is stored.
	contentCacheDir string
	// Metadata contains select image attributes
	Metadata Metadata
	// Layers contains the rich layer objects in build order
	Layers []*Layer
	// FileCatalog contains all file metadata for all files in all layers
	FileCatalog FileCatalogReader

	SquashedSearchContext filetree.Searcher

	overrideMetadata []AdditionalMetadata

	// wrapLazyLayerTree is a test hook: when set, each layer tree handed to a lazy layer squash goes through
	// it, so a test can see (or break) the build that reads it
	wrapLazyLayerTree func(layerIdx int, layerTree filetree.ReadWriter) filetree.ReadWriter
}

type AdditionalMetadata func(*Image) error

func WithTags(tags ...string) AdditionalMetadata {
	return func(image *Image) error {
		existingTags := strset.New()
		for _, t := range image.Metadata.Tags {
			existingTags.Add(t.String())
		}

		for _, t := range tags {
			// it is possible that we are given references that have both a tag and a digest (or only one)
			// we should only be allowing tags (stripping off digests if they are present)
			fields := strings.Split(t, "@")
			withNoDigest := fields[0]
			if !strings.Contains(withNoDigest, ":") {
				continue
			}
			tagObj, err := name.NewTag(withNoDigest)
			if err != nil {
				log.Warnf("unable to parse additional image tag to add %q: %+v", t, err)
				continue
			}
			if !existingTags.Has(tagObj.String()) {
				image.Metadata.Tags = append(image.Metadata.Tags, tagObj)
			}
		}
		return nil
	}
}

func WithManifest(manifest []byte) AdditionalMetadata {
	return func(image *Image) error {
		image.Metadata.RawManifest = manifest
		image.Metadata.ManifestDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))
		return nil
	}
}

func WithManifestDigest(digest string) AdditionalMetadata {
	return func(image *Image) error {
		image.Metadata.ManifestDigest = digest
		return nil
	}
}

func WithConfig(config []byte) AdditionalMetadata {
	return func(image *Image) error {
		image.Metadata.RawConfig = config
		image.Metadata.ID = fmt.Sprintf("sha256:%x", sha256.Sum256(config))
		return nil
	}
}

func WithRepoDigests(digests ...string) AdditionalMetadata {
	return func(image *Image) error {
		image.Metadata.RepoDigests = append(image.Metadata.RepoDigests, digests...)
		return nil
	}
}

func WithPlatform(platform string) AdditionalMetadata {
	return func(image *Image) error {
		p, err := NewPlatform(platform)
		if err != nil {
			return err
		}
		image.Metadata.Architecture = p.Architecture
		image.Metadata.Variant = p.Variant
		image.Metadata.OS = p.OS
		return nil
	}
}

func WithArchitecture(architecture, variant string) AdditionalMetadata {
	return func(image *Image) error {
		if architecture == "" {
			return nil
		}
		if !isKnownArch(architecture) {
			return fmt.Errorf("unknown architecture: %s", architecture)
		}
		image.Metadata.Architecture = architecture
		image.Metadata.Variant = variant
		return nil
	}
}

func WithOS(o string) AdditionalMetadata {
	return func(image *Image) error {
		if o == "" {
			return nil
		}
		if !isKnownOS(o) {
			return fmt.Errorf("unknown OS: %s", o)
		}
		image.Metadata.OS = o
		return nil
	}
}

// NewImage provides a new (unread) image object.
//
// Deprecated: use New() instead
func NewImage(image v1.Image, tmpDirGen *file.TempDirGenerator, contentCacheDir string, additionalMetadata ...AdditionalMetadata) *Image {
	return New(image, tmpDirGen, contentCacheDir, additionalMetadata...)
}

// New provides a new (unread) image object.
func New(image v1.Image, tmpDirGen *file.TempDirGenerator, contentCacheDir string, additionalMetadata ...AdditionalMetadata) *Image {
	imgObj := &Image{
		image:            image,
		tmpDirGen:        tmpDirGen,
		contentCacheDir:  contentCacheDir,
		overrideMetadata: additionalMetadata,
	}
	return imgObj
}

func (i *Image) IDs() []string {
	var ids = make([]string, len(i.Metadata.Tags))
	for idx, t := range i.Metadata.Tags {
		ids[idx] = t.String()
	}
	ids = append(ids, i.Metadata.ID)
	return ids
}

func (i *Image) trackReadProgress(metadata Metadata) *progress.Manual {
	prog := progress.NewManual(
		// x2 for read and squash of each layer
		int64(len(metadata.Config.RootFS.DiffIDs) * 2),
	)

	bus.Publish(partybus.Event{
		Type:   event.ReadImage,
		Source: metadata,
		Value:  progress.Progressable(prog),
	})

	return prog
}

func (i *Image) applyOverrideMetadata() error {
	for _, optionFn := range i.overrideMetadata {
		if err := optionFn(i); err != nil {
			return fmt.Errorf("unable to override metadata option: %w", err)
		}
	}
	return nil
}

// Read parses information from the underlying image tar into this struct. This includes image metadata, layer
// metadata, layer file trees, and layer squash trees (which implies the image squash tree). Under
// WithLazyLayerSquashes only the top layer's squash, the image squash, is built here, and every lower
// layer's squash is built the first time something uses it.
//
// The context bounds the read: cancelling it abandons work that has not started and stops the
// layer pools from picking up more. It does not return until the layer work already in flight has
// finished, so nothing it releases on the way out is still being written to - neither the layers
// themselves nor i.Metadata, which the index stage accumulates into.
//
// How long that wait is depends on the source. A registry fetch aborts with the caller's context,
// but a daemon or archive fetch reads a local tar through go-containerregistry, which takes no
// context, and indexing a layer is never interruptible. So a cancelled read costs up to one fetch
// plus one index per in-flight worker slot, plus the one layer squash in progress.
func (i *Image) Read(ctx context.Context) error {
	startTime := time.Now()

	if err := i.readMetadata(); err != nil {
		return err
	}

	layers, err := i.buildLayers()
	if err != nil {
		return err
	}

	log.WithFields("digest", i.Metadata.ID, "mediaType", i.Metadata.MediaType, "tags", i.Metadata.Tags).Debug("reading image")

	// let consumers know of a monitorable event (image save + copy stages)
	readProg := i.trackReadProgress(i.Metadata)
	// deferred rather than completed at the end of the squash: every early return between here and
	// there used to leave a consumer's bar stuck at whatever it had reached
	defer readProg.SetCompleted()

	// this rebuilds every layer, so release what a previous Read left open. Done here rather than
	// at the top of Read so that a failure before this point leaves the existing layers usable
	i.releasePreviousLayers()

	fileCatalog := NewFileCatalog()

	lapTime := time.Now()
	if err := i.readAndSquashLayers(ctx, layers, fileCatalog, readProg); err != nil {
		// i.Layers is not assigned until this succeeds, so a failure releases the local slice
		// rather than i.Layers - the caller has an error and may never reach Cleanup, and a
		// half-built layer set must not be left visible either: accessors like SquashedTree read
		// the last layer, which here was never squashed.
		for _, closeErr := range closeLayers(layers) {
			log.WithFields("error", closeErr).Trace("unable to release a layer tar after a failed read")
		}
		return err
	}

	log.WithFields("digest", i.Metadata.ID, "time", time.Since(lapTime)).Trace("completed image layer read and squash")
	lapTime = time.Now()

	i.Layers = layers
	i.FileCatalog = fileCatalog
	i.SquashedSearchContext = i.squashedSearchContext()

	log.WithFields("digest", i.Metadata.ID, "time", time.Since(lapTime)).Trace("completed image search context")
	log.WithFields("digest", i.Metadata.ID, "mediaType", i.Metadata.MediaType, "tags", i.Metadata.Tags, "time", time.Since(startTime)).Info("completed image read")

	return nil
}

// readMetadata populates i.Metadata from the image, then lets the caller's overrides have the
// final word on it.
func (i *Image) readMetadata() error {
	var err error
	i.Metadata, err = readImageMetadata(i.image)
	if err != nil {
		return err
	}
	return i.applyOverrideMetadata()
}

// buildLayers resolves the layer set this read will work through, rejecting media types we cannot
// process up front rather than partway through a read of an image we were never going to finish.
func (i *Image) buildLayers() ([]*Layer, error) {
	v1Layers, err := i.image.Layers()
	if err != nil {
		return nil, err
	}
	if err := validateLayerMediaTypes(v1Layers); err != nil {
		return nil, err
	}
	return newLayers(v1Layers, i.Metadata.Config.RootFS.DiffIDs), nil
}

// releasePreviousLayers releases every layer tar a previous Read left open and forgets the layers.
func (i *Image) releasePreviousLayers() {
	for _, closeErr := range i.closeLayers() {
		log.WithFields("error", closeErr).Trace("unable to release a layer tar from a previous read")
	}
	i.Layers = nil
}

// readAndSquashLayers runs the squash alongside the read stages rather than after them: squash
// consumes layers in manifest order, blocking on each layer's gate until that layer is indexed, so
// layer 0 is being squashed while layer 3 is still downloading. See readLayers for how the read
// stages are bounded and squashLayers for why consuming in manifest order keeps it correct.
func (i *Image) readAndSquashLayers(ctx context.Context, layers []*Layer, fileCatalog *FileCatalog, readProg *progress.Manual) error {
	gates := newLayerGates(len(layers))
	squashDone := make(chan error, 1)
	// capture what the goroutine logs, and send last: once Read receives from squashDone it may
	// return, and a caller is free to call Read again, which rewrites i.Metadata
	squashDigest := i.Metadata.ID
	go func() {
		squashStart := time.Now()
		err := i.squashLayers(ctx, layers, gates, readProg, !lazyLayerSquashes(ctx))
		log.WithFields("digest", squashDigest, "time", time.Since(squashStart)).Trace("completed image squash")
		squashDone <- err
	}()

	readErr := i.readLayers(ctx, layers, fileCatalog, readProg, gates)

	// release any layer the read never reached, so squash cannot block on a gate that will
	// never open (a cancelled context stops the stages from starting queued work)
	gates.releaseAll()
	squashErr := <-squashDone

	if readErr != nil {
		return readErr
	}
	if err := ctx.Err(); err != nil {
		// checked before squashErr because cancellation is the root cause a caller wants to see:
		// the stages stop starting queued work, so neither raises a per-layer failure and the
		// squash just finds layers that were never indexed
		return fmt.Errorf("unable to read image: %w", err)
	}
	return squashErr
}

// squashedSearchContext returns the search context for the image squash. The top layer's squash IS
// the image squash, and squashLayers already built a search context over that same tree and index.
// Rebuilding it here walked every symlink and hardlink in the whole catalog a second time, which
// with the squash now overlapped was the largest piece of serial work left in Read.
func (i *Image) squashedSearchContext() filetree.Searcher {
	if len(i.Layers) > 0 {
		return i.Layers[len(i.Layers)-1].SquashedSearchContext
	}
	return filetree.NewSearchContext(i.SquashedTree(), i.FileCatalog)
}

// readLayers drives the two stages of a layer read: a fetch stage (download + decompress into the
// cache) and an index stage (tar walk into the file catalog) that runs alongside it, so the
// previous layer is indexed while the next is still being fetched.
//
// Each stage is bounded by its own go-sync executor pulled from the context, which is what lets a
// caller fold stereoscope into a process-wide concurrency budget: install executors under
// LayerFetchExecutor and LayerIndexExecutor and they win. Read fills in the default for whichever
// stage the caller left out, so the two bounds stay independent - the registry provider installs a
// fetch executor of one and leaves indexing at the default, which a single fused bound cannot
// express.
//
// Errors from both stages are joined. Panics inside either stage are captured as errors rather
// than taking down the process. A cancelled context stops either stage from starting more work
// and is waited out rather than abandoned, and the first layer failure sets an internal flag that
// does the same to its peers' fetches - no sense downloading layer 7 after layer 2 is already
// known to be corrupt.
func (i *Image) readLayers(ctx context.Context, layers []*Layer, fileCatalog *FileCatalog, readProg *progress.Manual, gates *layerGates) error {
	ctx = withLayerExecutors(ctx, len(layers))

	// errors are recorded by layer index and reported in that order. Both stages can run several
	// layers before either one fails, so more than one can fail, and go-sync joins its own errors
	// in completion order - which would name a different layer first from one run to the next.
	//
	// aborted is deliberately not a cancelled context: ctx keeps meaning only "the caller gave
	// up". Cancelling an internal context here would let Collect abandon a peer's fetch mid-flight
	// before that peer's own recordErr call - the very call whose result we are about to read -
	// had a chance to run (see the fetchCtx comment below). aborted only stops fetchIdxs from
	// handing out layers nothing has started on yet; anything already dispatched runs to
	// completion.
	var (
		errMu     sync.Mutex
		layerErrs = map[int]error{}
		aborted   atomic.Bool
	)
	recordErr := func(idx int, err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if _, seen := layerErrs[idx]; !seen {
			layerErrs[idx] = err
		}
		aborted.Store(true)
	}
	orderedErrs := func() []error {
		errMu.Lock()
		defer errMu.Unlock()
		idxs := slices.Sorted(maps.Keys(layerErrs))
		out := make([]error, 0, len(idxs))
		for _, idx := range idxs {
			out = append(out, layerErrs[idx])
		}
		return out
	}

	idxs := make([]int, len(layers))
	for n := range idxs {
		idxs[n] = n
	}

	// fetchIdxs stops handing out new layer indices once aborted is set or the caller has given
	// up, so Collect never starts a fetch for a layer we already know we will not need. Whatever
	// it already started keeps running to completion regardless - see the aborted comment above,
	// and the stage contexts below, for why that matters.
	fetchIdxs := func(yield func(int) bool) {
		for _, idx := range idxs {
			if aborted.Load() || ctx.Err() != nil {
				return
			}
			if !yield(idx) {
				return
			}
		}
	}

	// fetched hands layer indexes from the fetch stage to the index stage, buffered so that a send
	// can never block: there are at most len(layers) of them.
	//
	// Do not shrink this to bound the fetch-ahead. The two stages can share one bounded executor -
	// go-sync resolves a missing named executor to ExecutorDefault, and errGroupExecutor.Go blocks
	// once its limit is reached, and its ChildExecutor caches a single child that both stages then
	// resolve to. A fetch worker parked on a full channel is holding a slot the index stage needs
	// to drain it, and with a limit of one that would be a deadlock. Bounding the
	// fetch-ahead has to happen where no worker slot is held: in fetchIdxs, which runs on the
	// submitting goroutine, gated on how many layers are fetched but not yet indexed.
	fetched := make(chan int, len(layers))
	fetchDone := make(chan error, 1)

	// neither stage hands the caller's context to Collect: go-sync v0.1.2 Collect returns as soon
	// as its context is done, abandoning workers it already dispatched mid-write into layers that
	// Read then closes. An uncancellable context leaves it only its wait, so its return means
	// every worker finished. WithoutCancel rather than Background because it keeps the context
	// values, and that is where a caller's stage executors live - Background would compile, read
	// the same, and silently drop the registry provider's deliberate fetch bound of one.
	//
	// Cancellation is still honoured where acting on it is safe: the iterators stop handing out
	// layers, and each task below returns immediately when ctx is already done, which is what
	// replaces Collect's own pre-task check. What is in flight always runs to completion.
	fetchCtx := context.WithoutCancel(ctx)
	go func() {
		err := async.Collect(&fetchCtx, LayerFetchExecutor, fetchIdxs,
			func(idx int) (int, error) {
				if ctx.Err() != nil {
					gates.done(idx, false)
					return idx, nil
				}
				if err := layers[idx].fetch(idx, i.contentCacheDir); err != nil {
					// the index stage will never see this layer, so open its gate here
					gates.done(idx, false)
					// reported through recordErr rather than back to Collect, so that what Collect
					// returns is only ever a panic it recovered
					recordErr(idx, fmt.Errorf("failed to fetch layer %d: %w", idx, err))
					return idx, nil
				}
				// cannot block: fetched is buffered to len(layers) and there are at most that
				// many sends
				fetched <- idx
				return idx, nil
			}, nil)
		// safe unconditionally: fetchCtx cannot be cancelled, so Collect only returns once every
		// fetch worker has, and there is no sender left to panic on a closed channel
		close(fetched)
		fetchDone <- err
	}()

	indexCtx := context.WithoutCancel(ctx)
	indexErr := async.Collect(&indexCtx, LayerIndexExecutor, seqOfChannel(ctx, fetched),
		func(idx int) (int, error) {
			if ctx.Err() != nil {
				gates.done(idx, false)
				return idx, nil
			}
			err := layers[idx].index(fileCatalog)
			// open the gate either way: squash decides what to do with the outcome
			gates.done(idx, err == nil)
			if err != nil {
				recordErr(idx, fmt.Errorf("failed to index layer %d: %w", idx, err))
				return idx, nil
			}
			readProg.Increment()
			return idx, nil
		},
		// the accumulator is serialised by Collect, so this needs no lock of its own
		func(idx int, _ int) {
			i.Metadata.Size += layers[idx].Metadata.Size
		})

	// fetchDone is read before orderedErrs, not inline in the Join call: Collect only guarantees
	// the fetch stage's own recordErr calls are visible once it has actually signalled done, and
	// argument evaluation order would otherwise let orderedErrs run first and race them.
	fetchErr := <-fetchDone
	// what the stages report, lowest layer first, then anything go-sync recovered on its own
	return errors.Join(append(orderedErrs(), fetchErr, indexErr)...)
}

// LayerFetchExecutor and LayerIndexExecutor name the go-sync executors bounding each stage of a
// layer read. Install one under either name to take over that stage's concurrency:
//
//	ctx = sync.SetContextExecutor(ctx, image.LayerFetchExecutor, sync.NewExecutor(2))
//
// A caller-supplied executor always wins; Read only fills in what is missing. It must run every
// function handed to Go: Read waits for in-flight workers rather than abandoning them, so an
// executor that silently drops work will hang the read.
const (
	LayerFetchExecutor = "layer-fetch"
	LayerIndexExecutor = "layer-index"
)

type lazyLayerSquashesKey struct{}

// WithLazyLayerSquashes returns a context that tells Read to build only the image squash (the top
// layer's), and to build every lower layer's squash the first time something uses it. By default Read
// builds every layer's squash, and since each one is a full copy of the tree below it, memory grows
// with layers x files. Use this when you expect to need only the image squash (for example a
// squashed-scope scan): lower layers then cost nothing.
//
// Lower layers still work, Layer.SquashedTree and Layer.SquashedSearchContext are never nil, but:
//   - the first call that touches a lower layer builds its squash, and the squash of every layer below
//     it that has not been built yet. That call can take a while and cannot be cancelled, since no
//     context reaches it. Concurrent callers wait for the same build rather than repeating it.
//   - a failed build no longer fails Read. Methods that return an error report it, and the rest see
//     an empty tree (see filetree.NewLazySquash). A caller that must not miss files (a scanner) should
//     read lower layers through the methods that return an error: SquashedSearchContext, and
//     SquashedTree's File, FilesByGlob, ListPaths and Walk, rather than AllFiles or HasPath.
//   - a lower layer's SquashedTree is not a *filetree.FileTree, so type assertions on it fail.
//
// It applies to every Read under the returned context, so set it on a context scoped to the reads
// that want it.
func WithLazyLayerSquashes(ctx context.Context) context.Context {
	return context.WithValue(ctx, lazyLayerSquashesKey{}, true)
}

func lazyLayerSquashes(ctx context.Context) bool {
	v, _ := ctx.Value(lazyLayerSquashesKey{}).(bool)
	return v
}

// withLayerExecutors fills in the stage executors this image should use for any the caller has not
// supplied. Never bounded at zero: a zero-concurrency go-sync executor runs inline on the caller's
// goroutine, which would collapse the two stages back into one and lose the overlap.
func withLayerExecutors(ctx context.Context, layers int) context.Context {
	for _, name := range []string{LayerFetchExecutor, LayerIndexExecutor} {
		if async.HasContextExecutor(ctx, name) {
			continue
		}
		// go-sync resolves a missing named executor to ExecutorDefault, so a host that installed
		// one to express a single process-wide budget already has an answer for this stage and we
		// should not talk over it. Only fill in when there is nothing at all, because the last
		// fallback go-sync offers is an inline serial executor, which would collapse the two
		// stages into one and lose the overlap.
		if async.HasContextExecutor(ctx, async.ExecutorDefault) {
			continue
		}
		ctx = async.SetContextExecutor(ctx, name, async.NewExecutor(layerReadWorkers(layers)))
	}
	return ctx
}

// layerGates lets the squash stage consume layers in manifest order while the read stages
// complete them in any order: squash blocks on gate idx until the read stages are done with that
// layer, and learns from the gate whether it is safe to squash.
//
// Release is idempotent so the read stages and the caller's cleanup pass can both call it. A
// layer the read never reached is released as not-ok, which is what stops squash blocking
// forever on a cancelled read.
type layerGates struct {
	wg   []sync.WaitGroup
	once []sync.Once
	ok   []atomic.Bool
}

func newLayerGates(layers int) *layerGates {
	g := &layerGates{
		wg:   make([]sync.WaitGroup, layers),
		once: make([]sync.Once, layers),
		ok:   make([]atomic.Bool, layers),
	}
	for idx := range g.wg {
		g.wg[idx].Add(1)
	}
	return g
}

// done opens a layer's gate, reporting whether that layer is indexed and safe to squash.
func (g *layerGates) done(idx int, ok bool) {
	g.once[idx].Do(func() {
		g.ok[idx].Store(ok)
		g.wg[idx].Done()
	})
}

// wait blocks until the layer's gate opens and reports whether it is safe to squash.
func (g *layerGates) wait(idx int) bool {
	g.wg[idx].Wait()
	return g.ok[idx].Load()
}

// releaseAll opens every gate that is still closed, so nothing is left waiting on a layer the
// read stages never got to.
func (g *layerGates) releaseAll() {
	for idx := range g.wg {
		g.done(idx, false)
	}
}

// seqOfChannel adapts a channel to an iter.Seq so a go-sync Collect can consume a stage's output
// as it is produced rather than waiting for all of it.
//
// It watches ctx directly, and that is load-bearing: the index stage runs Collect on a context
// that cannot be cancelled, so Collect's own pre-queue check never fires and this is the only
// thing that stops indexing early. Do not drop the ctx case.
func seqOfChannel[T any](ctx context.Context, ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				if !yield(v) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

// layerReadWorkers is the default bound for a layer-read stage when neither a named executor nor
// ExecutorDefault is in the context: the number of CPUs, capped at 8, and never more workers than
// there are layers.
//
// The cap is measured, not a guess. Both stages are throughput-bound rather than latency-bound -
// decompress and write, then read back and walk - so oversubscribing past a handful of workers
// costs more in contention than it buys in overlap. On a 40-layer image over 12 cores, where the
// cap actually binds: 4 workers 718ms, 8 workers 679ms, 12 workers 694ms, 24 workers 715ms,
// 48 workers 746ms. Going wider is slower, so this is not the place to scale with NumCPU alone.
// Layer count is usually the real ceiling anyway; a 12-layer image cannot use more than 12.
func layerReadWorkers(layers int) int {
	n := runtime.NumCPU()
	if n > 8 {
		n = 8
	}
	if n > layers {
		n = layers
	}
	if n < 1 {
		n = 1
	}
	return n
}

// squash generates a squash tree for each layer in the image. For instance, layer 2 squash =
// squash(layer 0, layer 1, layer 2), layer 3 squash = squash(layer 0, layer 1, layer 2, layer 3), and so on.
//
// With eager set every layer's squash is built here, each onto a copy of the one below. Otherwise only
// the top layer's squash (the image squash) is built here, merged in place since no layer below it keeps
// that tree, and every lower layer gets a lazy squash that is built from its own chain on first use.
func (i *Image) squashLayers(ctx context.Context, layers []*Layer, gates *layerGates, prog *progress.Manual, eager bool) error {
	// eager: the squash of the layer below. Lazy: the image squash so far, merged into in place.
	var lastSquashTree filetree.ReadWriter
	// lazy only: the lazy squash of the layer below, which the next lazy squash is built from
	var lowerLazySquash filetree.Reader

	for idx, layer := range layers {
		if !gates.wait(idx) {
			// the read stages gave up on this layer, so there is no tree to squash. Report it
			// rather than returning nil and trusting the read stages to raise something: if their
			// error is ever lost, a nil return here lets Read carry on and build the image search
			// context over a layer that was never squashed, which panics on the nil tree.
			return fmt.Errorf("unable to squash: layer %d was not indexed", idx)
		}
		// checked per layer since the squash keeps running after the last layer is indexed, and Read
		// waits for it, so without this a cancel can't stop the copies left to do
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("unable to squash layer %d: %w", idx, err)
		}

		// before the merge. Only Windows layers link into the layers below, and only hcsshim imports them, so
		// that is the one model followed; a Linux image's links stay as their headers describe them. The
		// config is part of the image digest, so this stays a function of the image
		var below []*Layer
		if i.Metadata.Config.OS == "windows" {
			below = layers[:idx]
		}
		if err := adoptLowerLayerHardLinks(below, layer); err != nil {
			return fmt.Errorf("failed to adopt hardlinks in layer %d: %w", idx, err)
		}

		layerTree := layer.Tree.(filetree.ReadWriter)
		squashedTree, err := squashOnto(lastSquashTree, layerTree, eager)
		if err != nil {
			return fmt.Errorf("failed to squash tree %d: %w", idx, err)
		}
		lastSquashTree = squashedTree

		if eager || idx == len(layers)-1 {
			layer.SquashedTree = squashedTree
		} else {
			// never the in-place tree above: it keeps changing as later layers are merged into it
			lazyLayerTree := layerTree
			if i.wrapLazyLayerTree != nil {
				lazyLayerTree = i.wrapLazyLayerTree(idx, layerTree)
			}
			layer.SquashedTree = filetree.NewLazySquash(lowerLazySquash, lazyLayerTree)
			lowerLazySquash = layer.SquashedTree
		}
		// for a lazy squash this is lazy too, and builds nothing until the first search
		layer.SquashedSearchContext = filetree.NewSearchContext(layer.SquashedTree, layer.fileCatalog.Index)

		prog.Increment()
	}

	return nil
}

// squashOnto applies layerTree onto the squash below it (nil for the lowest layer). The lower squash
// is left untouched when keepLower is set, otherwise it is merged into and returned.
func squashOnto(lower, layerTree filetree.ReadWriter, keepLower bool) (filetree.ReadWriter, error) {
	if lower != nil && !keepLower {
		// a squash never carries whiteouts, so this is exactly what Squash would do after copying it
		return lower, lower.Merge(layerTree)
	}

	// the lowest layer has nothing under it to merge into, but it is still squashed rather than
	// used as-is: a layer tree keeps the whiteout entries of its own changeset, a squashed tree
	// must not carry them
	unionTree := filetree.NewUnionFileTree()
	if lower != nil {
		unionTree.PushTree(lower)
	}
	unionTree.PushTree(layerTree)
	return unionTree.Squash()
}

// SquashedTree returns the pre-computed image squash file tree.
func (i *Image) SquashedTree() filetree.Reader {
	layerCount := len(i.Layers)

	if layerCount == 0 {
		return filetree.New()
	}

	topLayer := i.Layers[layerCount-1]
	return topLayer.SquashedTree
}

// OpenPathFromSquash fetches file contents for a single path, relative to the image squash tree.
// If the path does not exist an error is returned.
func (i *Image) OpenPathFromSquash(path file.Path) (io.ReadCloser, error) {
	return fetchReaderByPath(i.SquashedTree(), i.FileCatalog, path)
}

// FileContentsFromSquash fetches file contents for a single path, relative to the image squash tree.
// If the path does not exist an error is returned.
//
// Deprecated: use OpenPathFromSquash() instead.
func (i *Image) FileContentsFromSquash(path file.Path) (io.ReadCloser, error) {
	return fetchReaderByPath(i.SquashedTree(), i.FileCatalog, path)
}

// FilesByMIMETypeFromSquash returns file references for files that match at least one of the given MIME types.
//
// Deprecated: please use SquashedSearchContext().SearchByMIMEType() instead.
func (i *Image) FilesByMIMETypeFromSquash(mimeTypes ...string) ([]file.Reference, error) {
	var refs []file.Reference
	refVias, err := i.SquashedSearchContext.SearchByMIMEType(mimeTypes...)
	if err != nil {
		return nil, err
	}
	for _, refVia := range refVias {
		if refVia.HasReference() {
			refs = append(refs, *refVia.Reference)
		}
	}
	return refs, nil
}

// OpenReference fetches file contents for a single file reference, regardless of the source layer.
// If the path does not exist an error is returned.
func (i *Image) OpenReference(ref file.Reference) (io.ReadCloser, error) {
	return i.FileCatalog.Open(ref)
}

// FileContentsByRef fetches file contents for a single file reference, regardless of the source layer.
// If the path does not exist an error is returned.
//
// Deprecated: please use OpenReference() instead.
func (i *Image) FileContentsByRef(ref file.Reference) (io.ReadCloser, error) {
	return i.FileCatalog.Open(ref)
}

// ResolveLinkByLayerSquash resolves a symlink for the given file reference relative to the result from
// the layer squash of the given layer index argument.
// If the given file reference is not a link type it resolves to itself. A dead link, or a link that is malformed in
// the image (a cycle, or a chain too deep to follow), resolves to nil with a nil error: callers are resolving one
// file out of many and one bad link should not fail the whole set.
func (i *Image) ResolveLinkByLayerSquash(ref file.Reference, layer int, options ...filetree.LinkResolutionOption) (*file.Resolution, error) {
	if i.Layers[layer].SquashedTree == nil {
		return nil, i.Layers[layer].errNoSquash()
	}
	allOptions := append([]filetree.LinkResolutionOption{filetree.FollowBasenameLinks}, options...)
	_, resolvedRef, err := i.Layers[layer].SquashedTree.File(ref.RealPath, allOptions...)
	if filetree.IsUnresolvableLink(err) {
		log.WithFields("path", ref.RealPath, "error", err).Trace("unable to resolve link in image, skipping")
		return nil, nil
	}
	return resolvedRef, err
}

// ResolveLinkByImageSquash resolves a symlink for the given file reference relative to the result from the image squash.
// If the given file reference is not a link type it resolves to itself. A dead link, or a link that is malformed in
// the image (a cycle, or a chain too deep to follow), resolves to nil with a nil error, see ResolveLinkByLayerSquash.
func (i *Image) ResolveLinkByImageSquash(ref file.Reference, options ...filetree.LinkResolutionOption) (*file.Resolution, error) {
	allOptions := append([]filetree.LinkResolutionOption{filetree.FollowBasenameLinks}, options...)
	_, resolvedRef, err := i.Layers[len(i.Layers)-1].SquashedTree.File(ref.RealPath, allOptions...)
	if filetree.IsUnresolvableLink(err) {
		log.WithFields("path", ref.RealPath, "error", err).Trace("unable to resolve link in image, skipping")
		return nil, nil
	}
	return resolvedRef, err
}

// closeLayers releases every layer tar descriptor in the given slice. Free-standing so a layer
// set that never became i.Layers (an in-flight Read that failed) can be released too.
func closeLayers(layers []*Layer) []error {
	var errs []error
	for _, l := range layers {
		if err := l.close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// closeLayers releases every layer tar descriptor this image is holding open.
func (i *Image) closeLayers() []error {
	return closeLayers(i.Layers)
}

// Cleanup removes all temporary files created from parsing the image. Future calls to image will not function correctly after this call.
func (i *Image) Cleanup() error {
	if i == nil {
		return nil
	}
	// descriptors before dirs: on Windows an open handle makes RemoveAll fail outright
	errs := i.closeLayers()
	if i.tmpDirGen != nil {
		if err := i.tmpDirGen.Cleanup(); err != nil {
			errs = append(errs, err)
		}

		if i.contentCacheDir != "" {
			if _, err := os.Stat(i.contentCacheDir); !os.IsNotExist(err) {
				if err := os.RemoveAll(i.contentCacheDir); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
}
