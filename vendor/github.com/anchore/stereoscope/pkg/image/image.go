package image

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

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
// metadata, layer file trees, and layer squash trees (which implies the image squash tree).
func (i *Image) Read() error {
	var layers = make([]*Layer, 0)
	var err error
	i.Metadata, err = readImageMetadata(i.image)
	if err != nil {
		return err
	}

	// override any metadata with what the user has provided manually
	if err = i.applyOverrideMetadata(); err != nil {
		return err
	}

	startTime := time.Now()
	lapTime := startTime

	v1Layers, err := i.image.Layers()
	if err != nil {
		return err
	}

	// validate all layer media types before processing
	if err := validateLayerMediaTypes(v1Layers); err != nil {
		return err
	}

	log.WithFields("digest", i.Metadata.ID, "mediaType", i.Metadata.MediaType, "tags", i.Metadata.Tags).Debug("reading image")

	// let consumers know of a monitorable event (image save + copy stages)
	readProg := i.trackReadProgress(i.Metadata)

	fileCatalog := NewFileCatalog()

	for idx, v1Layer := range v1Layers {
		layer := NewLayer(v1Layer)
		err := layer.Read(fileCatalog, idx, i.contentCacheDir)
		if err != nil {
			return err
		}
		i.Metadata.Size += layer.Metadata.Size
		layers = append(layers, layer)

		readProg.Increment()
	}

	i.Layers = layers

	log.WithFields("digest", i.Metadata.ID, "time", time.Since(lapTime)).Trace("completed image layer copy")
	lapTime = time.Now()

	// in order to resolve symlinks all squashed trees must be available
	err = i.squash(readProg)

	log.WithFields("digest", i.Metadata.ID, "time", time.Since(lapTime)).Trace("completed image squash")
	lapTime = time.Now()

	i.FileCatalog = fileCatalog
	i.SquashedSearchContext = filetree.NewSearchContext(i.SquashedTree(), i.FileCatalog)

	log.WithFields("digest", i.Metadata.ID, "time", time.Since(lapTime)).Trace("completed image search context")
	log.WithFields("digest", i.Metadata.ID, "mediaType", i.Metadata.MediaType, "tags", i.Metadata.Tags, "time", time.Since(startTime)).Info("completed image read")

	return err
}

// squash generates a squash tree for each layer in the image. For instance, layer 2 squash =
// squash(layer 0, layer 1, layer 2), layer 3 squash = squash(layer 0, layer 1, layer 2, layer 3), and so on.
func (i *Image) squash(prog *progress.Manual) error {
	var lastSquashTree filetree.ReadWriter

	for idx, layer := range i.Layers {
		if idx == 0 {
			lastSquashTree = layer.Tree.(filetree.ReadWriter)
			layer.SquashedTree = layer.Tree
			layer.SquashedSearchContext = filetree.NewSearchContext(layer.SquashedTree, layer.fileCatalog.Index)
			continue
		}

		var unionTree = filetree.NewUnionFileTree()
		unionTree.PushTree(lastSquashTree)
		unionTree.PushTree(layer.Tree.(filetree.ReadWriter))

		squashedTree, err := unionTree.Squash()
		if err != nil {
			return fmt.Errorf("failed to squash tree %d: %w", idx, err)
		}

		layer.SquashedTree = squashedTree
		layer.SquashedSearchContext = filetree.NewSearchContext(layer.SquashedTree, layer.fileCatalog.Index)
		lastSquashTree = squashedTree

		prog.Increment()
	}

	prog.SetCompleted()

	return nil
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

// ResolveLinkByLayerSquash resolves a symlink or hardlink for the given file reference relative to the result from
// the layer squash of the given layer index argument.
// If the given file reference is not a link type, or is a unresolvable (dead) link, then the given file reference is returned.
func (i *Image) ResolveLinkByLayerSquash(ref file.Reference, layer int, options ...filetree.LinkResolutionOption) (*file.Resolution, error) {
	allOptions := append([]filetree.LinkResolutionOption{filetree.FollowBasenameLinks}, options...)
	_, resolvedRef, err := i.Layers[layer].SquashedTree.File(ref.RealPath, allOptions...)
	return resolvedRef, err
}

// ResolveLinkByImageSquash resolves a symlink or hardlink for the given file reference relative to the result from the image squash.
// If the given file reference is not a link type, or is a unresolvable (dead) link, then the given file reference is returned.
func (i *Image) ResolveLinkByImageSquash(ref file.Reference, options ...filetree.LinkResolutionOption) (*file.Resolution, error) {
	allOptions := append([]filetree.LinkResolutionOption{filetree.FollowBasenameLinks}, options...)
	_, resolvedRef, err := i.Layers[len(i.Layers)-1].SquashedTree.File(ref.RealPath, allOptions...)
	return resolvedRef, err
}

// Cleanup removes all temporary files created from parsing the image. Future calls to image will not function correctly after this call.
func (i *Image) Cleanup() error {
	if i == nil {
		return nil
	}
	var errs []error
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
