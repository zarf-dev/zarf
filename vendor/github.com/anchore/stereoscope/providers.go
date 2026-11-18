package stereoscope

import (
	"github.com/anchore/go-collections"
	containerdClient "github.com/anchore/stereoscope/internal/containerd"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/stereoscope/pkg/image/containerd"
	"github.com/anchore/stereoscope/pkg/image/containerstorage"
	"github.com/anchore/stereoscope/pkg/image/docker"
	"github.com/anchore/stereoscope/pkg/image/oci"
	"github.com/anchore/stereoscope/pkg/image/podman"
	"github.com/anchore/stereoscope/pkg/image/sif"
)

const (
	// FileTag marks providers that read a pre-existing archive/directory from disk (no daemon or store involved).
	FileTag = "file"
	// DirTag marks providers that read from an OCI directory layout on disk.
	DirTag = "dir"
	// DaemonTag marks providers that resolve images via a running local daemon (docker, podman, containerd).
	DaemonTag = "daemon"
	// PullTag marks providers usable for auto-resolution of a bare image reference (i.e. not a file/dir path):
	// daemon providers, local content-addressed stores (e.g. containers-storage), and registry pulls all carry
	// this tag. It does not imply network access on its own.
	PullTag = "pull"
	// RegistryTag marks providers that resolve images directly from an OCI/docker registry.
	RegistryTag = "registry"
)

// ImageProviderConfig is the user-configuration containing all configuration needed by stereoscope image providers
type ImageProviderConfig struct {
	UserInput string
	Platform  *image.Platform
	Registry  image.RegistryOptions
}

func ImageProviders(cfg ImageProviderConfig) []collections.TaggedValue[image.Provider] {
	tempDirGenerator := rootTempDirGenerator.NewGenerator()
	return []collections.TaggedValue[image.Provider]{
		// file providers
		taggedProvider(docker.NewArchiveProvider(tempDirGenerator, cfg.UserInput), FileTag),
		taggedProvider(oci.NewArchiveProviderWithPlatform(tempDirGenerator, cfg.UserInput, cfg.Platform), FileTag),
		taggedProvider(oci.NewDirectoryProviderWithPlatform(tempDirGenerator, cfg.UserInput, cfg.Platform), FileTag, DirTag),
		taggedProvider(sif.NewArchiveProvider(tempDirGenerator, cfg.UserInput), FileTag),

		// daemon providers
		taggedProvider(docker.NewDaemonProvider(tempDirGenerator, cfg.UserInput, cfg.Platform), DaemonTag, PullTag),
		taggedProvider(podman.NewDaemonProvider(tempDirGenerator, cfg.UserInput, cfg.Platform), DaemonTag, PullTag),
		taggedProvider(containerd.NewDaemonProvider(tempDirGenerator, cfg.Registry, containerdClient.Namespace(), cfg.UserInput, cfg.Platform), DaemonTag, PullTag),

		// daemonless local store providers (e.g. buildah / rootless podman); checked before the OCI registry so that
		// locally built images resolve before falling back to a remote pull. Tagged PullTag (not DaemonTag) since
		// there's no daemon involved, despite doing no network I/O itself; see PullTag's doc comment above.
		taggedProvider(containerstorage.NewProvider(tempDirGenerator, cfg.UserInput, cfg.Platform), PullTag),

		// registry providers
		taggedProvider(oci.NewRegistryProvider(tempDirGenerator, cfg.Registry, cfg.UserInput, cfg.Platform), RegistryTag, PullTag),
	}
}

func taggedProvider(provider image.Provider, tags ...string) collections.TaggedValue[image.Provider] {
	return collections.NewTaggedValue[image.Provider](provider, append([]string{provider.Name()}, tags...)...)
}

func allProviderTags() []string {
	return collections.TaggedValueSet[image.Provider]{}.Join(ImageProviders(ImageProviderConfig{})...).Tags()
}
