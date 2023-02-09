package packager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	v1name "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// Publish publishes the package to a registry
//
// This is a wrapper around the oras library
// and much of the code was adapted from the oras CLI - https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
//
// Authentication is handled via the Docker config file created w/ `docker login`
func (p *Packager) Publish() error {
	p.cfg.DeployOpts.PackagePath = p.cfg.PublishOpts.PackagePath
	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	if p.cfg.PublishOpts.RegistryURL == "docker.io" {
		// docker.io is commonly used, but not a valid registry URL
		p.cfg.PublishOpts.RegistryURL = "registry-1.docker.io"
	}
	ref, err := p.ref("")
	if err != nil {
		return fmt.Errorf("unable to create reference: %w", err)
	}

	paths := []string{
		filepath.Join(p.tmp.Base, "checksums.txt"),
		filepath.Join(p.tmp.Base, "zarf.yaml"),
		filepath.Join(p.tmp.Base, "sboms.tar.zst"),
	}
	componentTarballs, err := filepath.Glob(filepath.Join(p.tmp.Base, "components", "*.tar.zst"))
	if err != nil {
		return err
	}
	paths = append(paths, componentTarballs...)
	imagesLayers, err := filepath.Glob(filepath.Join(p.tmp.Base, "images", "*"))
	if err != nil {
		return err
	}
	paths = append(paths, imagesLayers...)

	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()
	message.HeaderInfof("📦 PACKAGE PUBLISH %s", ref.Name())
	err = p.publish(ref, paths, spinner)
	if err != nil {
		return fmt.Errorf("unable to publish package %s: %w", ref, err)
	}
	skeletonRef, err := p.ref("skeleton")
	if err != nil {
		return fmt.Errorf("unable to create reference: %w", err)
	}
	skeletonPaths := []string{}
	for idx, path := range paths {
		// remove paths from the images dir
		if !strings.HasPrefix(path, filepath.Join(p.tmp.Base, "images")) {
			skeletonPaths = append(skeletonPaths, paths[idx])
		}
	}
	message.HeaderInfof("📦 PACKAGE PUBLISH %s", skeletonRef.Name())
	err = p.publish(skeletonRef, skeletonPaths, spinner)
	if err != nil {
		return fmt.Errorf("unable to publish package %s: %w", skeletonRef, err)
	}

	return nil
}

func (p *Packager) publish(ref v1name.Reference, paths []string, spinner *message.Spinner) error {
	message.Debugf("Publishing package to %s", ref)
	spinner.Updatef("Publishing package to: %s", ref)
	ns := p.cfg.PublishOpts.Namespace
	name := ref.Context().Name()
	registry := ref.Context().RegistryStr()

	// For pushing to Docker Hub, we need to set the scope to the repository with pull+push actions, otherwise a 401 is returned
	scopes := []string{
		fmt.Sprintf("repository:%s/%s:pull,push", ns, name),
	}
	ctx := auth.WithScopes(context.Background(), scopes...)

	dst, err := remote.NewRepository(ref.String())
	if err != nil {
		return err
	}
	// load default Docker config file
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return err
	}
	if !cfg.ContainsAuth() {
		return errors.New("no docker config file found, run 'docker login'")
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = registry
	if registry == "registry-1.docker.io" {
		// Docker stores its credentials under the following key, otherwise credentials use the registry URL
		key = "https://index.docker.io/v1/"
	}

	authConf, err := configs[0].GetCredentialsStore(key).Get(key)
	if err != nil {
		return fmt.Errorf("unable to get credentials for %s: %w", key, err)
	}

	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}

	dst.Client = &auth.Client{
		Credential: auth.StaticCredential(registry, cred),
		Cache:      auth.NewCache(),
		// ForceAttemptOAuth2: true,
	}

	if p.cfg.PublishOpts.PlainHTTP {
		dst.PlainHTTP = true
	}

	store, err := file.New("")
	if err != nil {
		return err
	}
	defer store.Close()

	// Unless specified, an empty manifest config will be used: `{}`
	// which causes an error on Google Artifact Registry
	// to negate this, we create a simple manifest config with some build metadata
	manifestConfig := v1.ConfigFile{
		Architecture: p.cfg.Pkg.Build.Architecture,
		Author:       p.cfg.Pkg.Build.User,
		Variant:      "zarf-package",
	}
	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(p.tmp.Base, "config.json"), manifestConfigBytes, 0600)
	if err != nil {
		return err
	}
	manifestConfigPath := filepath.Join(p.tmp.Base, "config.json")
	manifestConfigDesc, err := store.Add(ctx, "config.json", ocispec.MediaTypeImageConfig, manifestConfigPath)
	if err != nil {
		return err
	}

	var descs []ocispec.Descriptor

	for _, path := range paths {
		name, err := filepath.Rel(p.tmp.Base, path)
		if err != nil {
			return err
		}

		var mediaType string
		if strings.HasSuffix(name, ".tar.zst") {
			mediaType = "application/vnd.zarf.package.layer.v1.tar+zstd"
		} else if strings.HasSuffix(name, ".tar.gz") {
			mediaType = "application/vnd.zarf.package.layer.v1.tar+gzip"
		} else if strings.HasSuffix(name, ".yaml") {
			mediaType = "application/vnd.zarf.package.layer.v1.yaml"
		} else if strings.HasSuffix(name, ".txt") {
			mediaType = "application/vnd.zarf.package.layer.v1.txt"
		} else if strings.HasSuffix(name, ".json") {
			mediaType = "application/vnd.zarf.package.layer.v1.json"
		} else {
			mediaType = "application/vnd.zarf.package.layer.v1.unknown"
		}

		desc, err := store.Add(ctx, name, mediaType, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}
	packOpts := oras.PackOptions{}
	packOpts.ConfigDescriptor = &manifestConfigDesc
	pack := func() (ocispec.Descriptor, error) {
		// note the empty string for the artifactType
		root, err := oras.Pack(ctx, store, "", descs, packOpts)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if err = store.Tag(ctx, root, root.Digest.String()); err != nil {
			return ocispec.Descriptor{}, err
		}
		return root, nil
	}

	copyOpts := oras.DefaultCopyOptions
	if p.cfg.PublishOpts.Concurrency > copyOpts.Concurrency {
		copyOpts.Concurrency = p.cfg.PublishOpts.Concurrency
	}
	copyOpts.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
		message.Debug("layer", desc.Digest.Hex()[:12], "exists")
		return nil
	}
	copyOpts.PostCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		message.SuccessF(desc.Digest.Hex()[:12])
		return nil
	}

	push := func(root ocispec.Descriptor) error {
		message.Debugf("root descriptor: %v\n", root)
		if tag := dst.Reference.Reference; tag == "" {
			err = oras.CopyGraph(ctx, store, dst, root, copyOpts.CopyGraphOptions)
		} else {
			_, err = oras.Copy(ctx, store, root.Digest.String(), dst, tag, copyOpts)
		}
		return err
	}

	// first attempt to do a ArtifactManifest push
	root, err := pack()
	if err != nil {
		return err
	}

	copyRootAttempted := false
	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		message.Infof(desc.Digest.Hex()[:12])
		if content.Equal(root, desc) {
			// copyRootAttempted helps track whether the returned error is
			// generated from copying root.
			copyRootAttempted = true
		}
		if preCopy != nil {
			return preCopy(ctx, desc)
		}
		return nil
	}

	// attempt to push the artifact manifest
	if err = push(root); err == nil {
		spinner.Updatef("Published: %s [%s]", ref, root.MediaType)
		message.SuccessF("Published: %s [%s]", ref, root.MediaType)
		message.SuccessF("Digest: %s", root.Digest)
		return nil
	}
	// log the error, the expected error is a 400 manifest invalid
	message.Debug(err)

	if !copyRootAttempted || root.MediaType != ocispec.MediaTypeArtifactManifest ||
		!isManifestUnsupported(err) {
		return fmt.Errorf(`failed to push artifact manifest, 
		was it during the copying of root? (%t)
		was the root mediaType an artifact manifest? (%t)
		was it because the registry does not support the artifact manifest mediaType? (%t)
		
		%w`, !copyRootAttempted, root.MediaType == ocispec.MediaTypeArtifactManifest, !isManifestUnsupported(err), err)
	}

	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	dst.SetReferrersCapability(false)

	// fallback to an ImageManifest push
	packOpts.PackImageManifest = true
	root, err = pack()
	if err != nil {
		return err
	}

	copyOpts.FindSuccessors = func(ctx context.Context, fetcher content.Fetcher, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if content.Equal(node, root) {
			// skip non-config
			content, err := content.FetchAll(ctx, fetcher, root)
			if err != nil {
				return nil, err
			}
			var manifest ocispec.Manifest
			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, err
			}
			return []ocispec.Descriptor{manifest.Config}, nil
		}
		// config has no successors
		return nil, nil
	}

	if err = push(root); err != nil {
		return err
	}
	spinner.Updatef("Published: %s [%s]", ref, root.MediaType)
	message.SuccessF("Published: %s [%s]", ref, root.MediaType)
	message.SuccessF("Digest: %s", root.Digest)
	return nil
}

func (p *Packager) ref(skeleton string) (v1name.Reference, error) {
	name := p.cfg.Pkg.Metadata.Name
	ver := p.cfg.Pkg.Build.Version
	arch := p.cfg.Pkg.Build.Architecture
	if len(skeleton) > 0 {
		arch = skeleton
	}
	ns := p.cfg.PublishOpts.Namespace
	registry := p.cfg.PublishOpts.RegistryURL
	ref, err := v1name.ParseReference(fmt.Sprintf("%s/%s/%s:%s-%s", registry, ns, name, ver, arch), v1name.StrictValidation)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// isManifestUnsupported returns true if the error is an unsupported artifact manifest error.
func isManifestUnsupported(err error) bool {
	var errResp *errcode.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusBadRequest {
		return false
	}

	var errCode errcode.Error
	if !errors.As(errResp, &errCode) {
		return false
	}

	// As of November 2022, ECR is known to return UNSUPPORTED error when
	// putting an OCI artifact manifest.
	switch errCode.Code {
	case errcode.ErrorCodeManifestInvalid, errcode.ErrorCodeUnsupported:
		return true
	}
	return false
}
