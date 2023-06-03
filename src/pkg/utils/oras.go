// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// OrasRemote is a wrapper around the Oras remote repository that includes a progress bar for interactive feedback.
type OrasRemote struct {
	*remote.Repository
	*remote.Registry
	context.Context
	Transport *Transport
}

// ZarfOCIManifest is a wrapper around the OCI manifest
//
// it includes the path to the index.json, oci-layout, and image blobs.
// as well as a few helper functions for locating layers and calculating the size of the layers.
type ZarfOCIManifest struct {
	ocispec.Manifest
	indexPath     string
	ociLayoutPath string
}

// Locate returns the descriptor for the layer with the given path.
func (m *ZarfOCIManifest) Locate(path string) ocispec.Descriptor {
	return Find(m.Layers, func(layer ocispec.Descriptor) bool {
		return layer.Annotations[ocispec.AnnotationTitle] == path
	})
}

// SumLayersSize returns the sum of the size of all the layers in the manifest.
func (m *ZarfOCIManifest) SumLayersSize() int64 {
	var sum int64
	for _, layer := range m.Layers {
		sum += layer.Size
	}
	return sum
}

// NewOrasRemote returns an oras remote repository client and context for the given url.
//
// Registry auth is handled by the Docker CLI's credential store and checked before returning the client
func NewOrasRemote(url string) (*OrasRemote, error) {
	ref, err := registry.ParseReference(strings.TrimPrefix(url, OCIURLPrefix))
	if err != nil {
		return &OrasRemote{}, fmt.Errorf("failed to parse OCI reference: %w", err)
	}
	o := &OrasRemote{}
	o.Context = context.TODO()
	// patch docker.io to registry-1.docker.io
	// this allows end users to use docker.io as an alias for registry-1.docker.io
	if ref.Registry == "docker.io" {
		ref.Registry = "registry-1.docker.io"
	}
	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return &OrasRemote{}, err
	}
	reg, err := remote.NewRegistry(ref.Registry)
	if err != nil {
		return &OrasRemote{}, err
	}
	reg.PlainHTTP = zarfconfig.CommonOptions.Insecure
	repo.PlainHTTP = zarfconfig.CommonOptions.Insecure
	authClient, err := o.withAuthClient(ref)
	if err != nil {
		return &OrasRemote{}, err
	}
	reg.Client = authClient
	repo.Client = authClient
	o.Registry = reg
	o.Repository = repo
	err = o.CheckAuth()
	if err != nil {
		return &OrasRemote{}, fmt.Errorf("unable to authenticate to %s: %s", ref.Registry, err.Error())
	}
	return o, nil
}

// withScopes returns a context with the given scopes.
//
// This is needed for pushing to Docker Hub.
func withScopes(ref registry.Reference) context.Context {
	// For pushing to Docker Hub, we need to set the scope to the repository with pull+push actions, otherwise a 401 is returned
	scopes := []string{
		fmt.Sprintf("repository:%s:pull,push", ref.Repository),
	}
	return auth.WithScopes(context.TODO(), scopes...)
}

// withAuthClient returns an auth client for the given reference.
//
// The credentials are pulled using Docker's default credential store.
func (o *OrasRemote) withAuthClient(ref registry.Reference) (*auth.Client, error) {
	message.Debugf("Loading docker config file from default config location: %s", config.Dir())
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return &auth.Client{}, err
	}
	if !cfg.ContainsAuth() {
		return &auth.Client{}, errors.New("no docker config file found, run 'zarf tools registry login --help'")
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = ref.Registry
	if key == "registry-1.docker.io" {
		// Docker stores its credentials under the following key, otherwise credentials use the registry URL
		key = "https://index.docker.io/v1/"
	}

	authConf, err := configs[0].GetCredentialsStore(key).Get(key)
	if err != nil {
		return &auth.Client{}, fmt.Errorf("unable to get credentials for %s: %w", key, err)
	}

	if authConf.ServerAddress != "" {
		o.Context = withScopes(ref)
	}

	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = zarfconfig.CommonOptions.Insecure

	o.Transport = NewTransport(transport, nil)

	client := &auth.Client{
		Credential: auth.StaticCredential(ref.Registry, cred),
		Cache:      auth.NewCache(),
		Client: &http.Client{
			Transport: o.Transport,
		},
	}
	client.SetUserAgent("zarf/" + zarfconfig.CLIVersion)

	return client, nil
}

// CheckAuth checks if the user is authenticated to the remote registry.
func (o *OrasRemote) CheckAuth() error {
	return o.Registry.Ping(o.Context)
}

// FetchRoot fetches the root manifest from the remote repository.
func (o *OrasRemote) FetchRoot() (*ZarfOCIManifest, error) {
	// get the manifest descriptor
	// ref.Reference can be a tag or a digest
	descriptor, err := o.Resolve(o.Context, o.Reference.Reference)
	if err != nil {
		return nil, err
	}

	// get the manifest itself
	pulled, err := content.FetchAll(o.Context, o, descriptor)
	if err != nil {
		return nil, err
	}
	manifest := ocispec.Manifest{}

	if err = json.Unmarshal(pulled, &manifest); err != nil {
		return nil, err
	}
	return &ZarfOCIManifest{
		Manifest:      manifest,
		indexPath:     filepath.Join("images", "index.json"),
		ociLayoutPath: filepath.Join("images", "oci-layout"),
	}, nil
}

// FetchManifest fetches the manifest with the given descriptor from the remote repository.
func (o *OrasRemote) FetchManifest(desc ocispec.Descriptor) (manifest *ZarfOCIManifest, err error) {
	bytes, err := o.FetchLayer(desc)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		return manifest, err
	}
	return manifest, nil
}

// FetchLayer fetches the layer with the given descriptor from the remote repository.
func (o *OrasRemote) FetchLayer(desc ocispec.Descriptor) (bytes []byte, err error) {
	bytes, err = content.FetchAll(o.Context, o, desc)
	if err != nil {
		return bytes, err
	}
	return bytes, nil
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *OrasRemote) FetchZarfYAML(manifest *ZarfOCIManifest) (pkg types.ZarfPackage, err error) {
	zarfYamlDescriptor := manifest.Locate(zarfconfig.ZarfYAML)
	if zarfYamlDescriptor.Digest == "" {
		return pkg, fmt.Errorf("unable to find %s in the manifest", zarfconfig.ZarfYAML)
	}
	zarfYamlBytes, err := o.FetchLayer(zarfYamlDescriptor)
	if err != nil {
		return pkg, err
	}
	err = goyaml.Unmarshal(zarfYamlBytes, &pkg)
	if err != nil {
		return pkg, err
	}
	return pkg, nil
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *OrasRemote) FetchImagesIndex(manifest *ZarfOCIManifest) (index *ocispec.Index, err error) {
	indexDescriptor := manifest.Locate(manifest.indexPath)
	indexBytes, err := o.FetchLayer(indexDescriptor)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(indexBytes, &index)
	if err != nil {
		return nil, err
	}
	return index, nil
}

// LayersFromPaths returns the descriptors for the given paths from the root manifest.
func (o *OrasRemote) LayersFromPaths(requestedPaths []string) (layers []ocispec.Descriptor, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return nil, err
	}
	for _, path := range requestedPaths {
		layers = append(layers, manifest.Locate(path))
	}
	return layers, nil
}

// LayersFromRequestedComponents returns the descriptors for the given components from the root manifest.
//
// It also retrieves the descriptors for all image layers that are required by the components.
//
// It also respects the `required` flag on components, and will retrieve all necessary layers for required components.
func (o *OrasRemote) LayersFromRequestedComponents(requestedComponents []string) (layers []ocispec.Descriptor, err error) {
	root, err := o.FetchRoot()
	if err != nil {
		return nil, err
	}

	pkg, err := o.FetchZarfYAML(root)
	if err != nil {
		return nil, err
	}
	images := []string{}
	tarballFormat := "%s.tar"
	for _, name := range requestedComponents {
		component := Find(pkg.Components, func(component types.ZarfComponent) bool {
			return component.Name == name
		})
		if component.Name == "" {
			return nil, fmt.Errorf("component %s does not exist in this package", name)
		}
		layers = append(layers, root.Locate(filepath.Join(zarfconfig.ZarfComponentsDir, fmt.Sprintf(tarballFormat, component.Name))))
	}
	for _, component := range pkg.Components {
		// If we requested this component, or it is required, we need to pull its images
		if SliceContains(requestedComponents, component.Name) || component.Required {
			images = append(images, component.Images...)
		}
		// If we did not request this component, but it is required, we need to pull it's tarball
		if !SliceContains(requestedComponents, component.Name) && component.Required {
			layers = append(layers, root.Locate(filepath.Join(zarfconfig.ZarfComponentsDir, fmt.Sprintf(tarballFormat, component.Name))))
		}
	}
	if len(images) > 0 {
		// Add the image index and the oci-layout layers
		layers = append(layers, root.Locate(root.indexPath))
		layers = append(layers, root.Locate(root.ociLayoutPath))
		// Append the sbom.tar layer if it exists
		sbomDescriptor := root.Locate(zarfconfig.ZarfSBOMTar)
		if sbomDescriptor.Digest != "" {
			layers = append(layers, sbomDescriptor)
		}
		index, err := o.FetchImagesIndex(root)
		if err != nil {
			return nil, err
		}
		for _, image := range images {
			manifestDescriptor := Find(index.Manifests, func(layer ocispec.Descriptor) bool {
				return layer.Annotations[ocispec.AnnotationBaseImageName] == image
			})
			manifest, err := o.FetchManifest(manifestDescriptor)
			if err != nil {
				return nil, err
			}
			// Add the manifest and the manifest config layers
			layers = append(layers, root.Locate(filepath.Join("images", "blobs", "sha256", manifestDescriptor.Digest.Encoded())))
			layers = append(layers, root.Locate(filepath.Join("images", "blobs", "sha256", manifest.Config.Digest.Encoded())))

			// Add all the layers from the manifest
			for _, layer := range manifest.Layers {
				layerPath := filepath.Join("images", "blobs", "sha256", layer.Digest.Encoded())
				layers = append(layers, root.Locate(layerPath))
			}
		}
	}
	message.Infof("Pulling %+v", layers)
	return layers, nil
}

// PullPackage pulls the package from the remote repository and saves it to the given path.
//
// layersToPull is an optional parameter that allows the caller to specify which layers to pull.
//
// The following layers will ALWAYS be pulled if they exist:
//   - zarf.yaml
//   - checksums.txt
//   - zarf.yaml.sig
func (o *OrasRemote) PullPackage(out string, concurrency int, layersToPull ...ocispec.Descriptor) error {
	isPartialPull := len(layersToPull) > 0
	ref := o.Reference
	message.Debugf("Pulling %s", ref.String())
	message.Infof("Pulling Zarf package from %s", ref)

	manifest, err := o.FetchRoot()
	if err != nil {
		return err
	}

	estimatedBytes := int64(0)
	if isPartialPull {
		for _, desc := range layersToPull {
			estimatedBytes += desc.Size
		}
		alwaysPull := []string{zarfconfig.ZarfYAML, zarfconfig.ZarfChecksumsTxt, zarfconfig.ZarfYAMLSignature}
		for _, path := range alwaysPull {
			desc := manifest.Locate(path)
			layersToPull = append(layersToPull, desc)
			estimatedBytes += desc.Size
		}
	} else {
		estimatedBytes = manifest.SumLayersSize()
	}
	estimatedBytes += manifest.Config.Size

	dst, err := file.New(out)
	if err != nil {
		return err
	}
	defer dst.Close()

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = concurrency
	copyOpts.OnCopySkipped = PrintLayerExists
	copyOpts.PostCopy = PrintLayerExists
	if isPartialPull {
		paths := []string{}
		for _, layer := range layersToPull {
			paths = append(paths, layer.Annotations[ocispec.AnnotationTitle])
		}
		copyOpts.FindSuccessors = func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			nodes, err := content.Successors(ctx, fetcher, desc)
			if err != nil {
				return nil, err
			}
			var ret []ocispec.Descriptor
			for _, node := range nodes {
				if SliceContains(paths, node.Annotations[ocispec.AnnotationTitle]) {
					ret = append(ret, node)
				}
			}
			return ret, nil
		}
	}

	// Create a thread to update a progress bar as we save the package to disk
	doneSaving := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go RenderProgressBarForLocalDirWrite(out, estimatedBytes, &wg, doneSaving, "Pulling Zarf package data")
	_, err = oras.Copy(o.Context, o.Repository, ref.String(), dst, ref.String(), copyOpts)
	if err != nil {
		return err
	}

	// Send a signal to the progress bar that we're done and wait for it to finish
	doneSaving <- 1
	wg.Wait()

	message.Debugf("Pulled %s", ref.String())
	message.Successf("Pulled %s", ref.String())
	return nil
}

// PrintLayerExists prints a success message to the console when a layer has been successfully published to a registry.
func PrintLayerExists(_ context.Context, desc ocispec.Descriptor) error {
	title := desc.Annotations[ocispec.AnnotationTitle]
	var format string
	if title != "" {
		format = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], First30last30(title))
	} else {
		format = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
	}
	message.Successf(format)
	return nil
}
