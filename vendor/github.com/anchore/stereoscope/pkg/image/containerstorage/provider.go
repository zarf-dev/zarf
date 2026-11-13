//go:build containers_image_openpgp

// Package containerstorage provides an image.Provider that resolves images from the local containers-storage store
// (as populated by buildah and rootless/rootful podman). It is only compiled when the containers_image_openpgp build
// tag is set, since it depends on the containers/image and containers/storage libraries. Without that tag a stub
// provider is used (see provider_stub.go) that reports the feature is not compiled in.
package containerstorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.podman.io/image/v5/copy"
	dockerarchive "go.podman.io/image/v5/docker/archive"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/signature"
	storagetransport "go.podman.io/image/v5/storage"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/stereoscope/pkg/image/docker"
)

// Source is the image.Source string used to explicitly select this provider (e.g. "containers-storage:localhost/myimage:latest").
const Source image.Source = image.ContainersStorageSource

// NewProvider creates a new provider able to resolve images from the current user's default containers-storage store.
// This is the store typically populated by buildah and rootless podman (e.g. ~/.local/share/containers/storage for
// rootless users, /var/lib/containers/storage for root). The provider relies on the containers/storage default store
// configuration for the current process/user; it does not probe alternate storage locations.
func NewProvider(tmpDirGen *file.TempDirGenerator, imageStr string, platform *image.Platform) image.Provider {
	return &containersStorageProvider{
		tmpDirGen: tmpDirGen,
		imageStr:  imageStr,
		platform:  platform,
	}
}

// containersStorageProvider is an image.Provider capable of resolving an image from the local containers-storage store
// by copying it into a temporary docker-archive and delegating to the docker archive provider.
type containersStorageProvider struct {
	tmpDirGen *file.TempDirGenerator
	imageStr  string
	platform  *image.Platform
}

func (p *containersStorageProvider) Name() string {
	return Source
}

// Provide resolves the configured image reference from the current user's default containers-storage store. When the
// image is not present (or the store is unavailable) a non-nil error is returned so that source auto-resolution can
// continue to the next provider (e.g. the OCI registry).
func (p *containersStorageProvider) Provide(ctx context.Context) (*image.Image, error) {
	store, err := openDefaultStore()
	if err != nil {
		return nil, err
	}
	return p.provideFromStore(ctx, store)
}

// provideFromStore performs the actual resolution against the given containers-storage store: it copies the image to a
// temporary docker-archive and reuses the docker archive provider to construct the final stereoscope image. It is
// separated from Provide so that store construction can be controlled directly in tests.
func (p *containersStorageProvider) provideFromStore(ctx context.Context, store storage.Store) (*image.Image, error) {
	startTime := time.Now()

	srcRef, err := storagetransport.Transport.ParseStoreReference(store, p.imageStr)
	if err != nil {
		return nil, fmt.Errorf("invalid containers-storage reference %q: %w", p.imageStr, err)
	}

	tempDir, err := p.tmpDirGen.NewDirectory("containers-storage-image")
	if err != nil {
		return nil, err
	}

	archivePath := filepath.Join(tempDir, "image.tar")
	destRef, err := dockerarchive.ParseReference(archivePath)
	if err != nil {
		return nil, fmt.Errorf("invalid docker-archive destination %q: %w", archivePath, err)
	}

	policyContext, err := newInsecurePolicyContext()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := policyContext.Destroy(); closeErr != nil {
			log.Debugf("failed to destroy containers-storage policy context: %v", closeErr)
		}
	}()

	log.WithFields("image", p.imageStr, "archive", archivePath).Trace("copying image from containers-storage to docker archive")

	sysCtx := p.systemContext(tempDir)
	if _, err := copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
		SourceCtx: sysCtx,
	}); err != nil {
		return nil, fmt.Errorf("failed to copy image from containers-storage: %w", err)
	}

	log.WithFields("image", p.imageStr, "time", time.Since(startTime)).Debug("copied image from containers-storage")

	// the docker-archive we generated above does not carry the store's tags, repo digests, or the config's
	// OS/architecture, so gather them directly from the store image and pass them through as additional metadata
	metadata := p.additionalMetadata(ctx, srcRef, sysCtx)

	// reuse the existing docker archive provider to construct the final stereoscope image from the generated tar
	return docker.NewArchiveProvider(p.tmpDirGen, archivePath, metadata...).Provide(ctx)
}

// additionalMetadata recovers metadata that does not survive the copy into a tagless docker-archive: tags and repo
// digests from the resolved store image, and OS/architecture/variant from the image config. Failures are non-fatal:
// we log and return whatever metadata could be gathered so the resulting image is still usable.
func (p *containersStorageProvider) additionalMetadata(ctx context.Context, srcRef types.ImageReference, sysCtx *types.SystemContext) []image.AdditionalMetadata {
	metadata := p.tagAndDigestMetadata(srcRef)
	return append(metadata, p.platformMetadata(ctx, srcRef, sysCtx)...)
}

// tagAndDigestMetadata resolves srcRef back against the store to recover the names (tags) and manifest digest
// recorded for the image. It resolves via srcRef (rather than re-parsing the raw, possibly non-normalized
// p.imageStr) so that inputs like a bare "myimage" or "localhost/myimage" without a tag still match the same
// normalized name the copy above used.
func (p *containersStorageProvider) tagAndDigestMetadata(srcRef types.ImageReference) (metadata []image.AdditionalMetadata) {
	_, storeImage, err := storagetransport.ResolveReference(srcRef)
	if err != nil {
		log.Warnf("unable to resolve containers-storage image %q for tags/digests: %v", p.imageStr, err)
		return nil
	}

	if len(storeImage.Names) > 0 {
		metadata = append(metadata, image.WithTags(storeImage.Names...))
	}
	if digests := repoDigests(storeImage); len(digests) > 0 {
		metadata = append(metadata, image.WithRepoDigests(digests...))
	}
	return metadata
}

// repoDigests derives docker-style "repo@digest" strings from the store image's names and canonical digest,
// mirroring the RepoDigests docker.NewDaemonProvider surfaces for daemon-resolved images. A name is skipped (rather
// than failing metadata gathering) if it doesn't parse as an image reference.
func repoDigests(storeImage *storage.Image) (digests []string) {
	if storeImage.Digest == "" {
		return nil
	}
	for _, name := range storeImage.Names {
		named, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			continue
		}
		canonical, err := reference.WithDigest(reference.TrimNamed(named), storeImage.Digest)
		if err != nil {
			continue
		}
		digests = append(digests, canonical.String())
	}
	return digests
}

// platformMetadata inspects the resolved image's config for the OS/architecture/variant, since the generated
// docker-archive does not carry it either.
func (p *containersStorageProvider) platformMetadata(ctx context.Context, srcRef types.ImageReference, sysCtx *types.SystemContext) (metadata []image.AdditionalMetadata) {
	img, err := srcRef.NewImage(ctx, sysCtx)
	if err != nil {
		log.Warnf("unable to open containers-storage image %q for platform metadata: %v", p.imageStr, err)
		return nil
	}
	defer func() {
		if closeErr := img.Close(); closeErr != nil {
			log.Debugf("failed to close containers-storage image source: %v", closeErr)
		}
	}()

	info, err := img.Inspect(ctx)
	if err != nil {
		log.Warnf("unable to inspect containers-storage image %q for platform metadata: %v", p.imageStr, err)
		return nil
	}

	if info.Architecture != "" {
		metadata = append(metadata, image.WithArchitecture(info.Architecture, info.Variant))
	}
	if info.Os != "" {
		metadata = append(metadata, image.WithOS(info.Os))
	}
	return metadata
}

// systemContext builds a containers/image SystemContext carrying the requested platform selection (if any) so that
// multi-arch images stored locally resolve to the requested OS/architecture/variant. bigFilesDir is used for any
// large-blob staging during the copy: containers/image otherwise hardcodes /var/tmp for this (to avoid a
// systemd-tmpfs /tmp), which isn't always writable; bigFilesDir is a directory already managed by our own
// TempDirGenerator and cleaned up alongside the rest of the resolved image's temp files.
func (p *containersStorageProvider) systemContext(bigFilesDir string) *types.SystemContext {
	sysCtx := &types.SystemContext{
		BigFilesTemporaryDir: bigFilesDir,
	}
	if p.platform != nil {
		sysCtx.OSChoice = p.platform.OS
		sysCtx.ArchitectureChoice = p.platform.Architecture
		sysCtx.VariantChoice = p.platform.Variant
	}
	return sysCtx
}

// openDefaultStore opens the containers-storage store described by the default configuration for the current
// process/user. It only opens a store that already exists on disk: storage.GetStore would otherwise silently
// create the graph/run root directories as a side effect of what is, for auto-resolution of a plain image
// reference, meant to be a read-only "is this available locally" check.
func openDefaultStore() (storage.Store, error) {
	storeOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to load default containers-storage options: %w", err)
	}

	if _, err := os.Stat(storeOptions.GraphRoot); err != nil {
		return nil, fmt.Errorf("no local containers-storage store found at %q: %w", storeOptions.GraphRoot, err)
	}

	store, err := storage.GetStore(storeOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to open containers-storage store: %w", err)
	}
	return store, nil
}

// newInsecurePolicyContext returns a policy context that accepts any image. This is appropriate because copying an
// image out of the local containers-storage store is a local data transfer, not a trust-validation operation.
func newInsecurePolicyContext() (*signature.PolicyContext, error) {
	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
	pc, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy context: %w", err)
	}
	return pc, nil
}
