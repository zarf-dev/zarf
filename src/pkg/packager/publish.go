package packager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// NOTES:
// - This is a WIP, not yet functional
// - This is a copy of the oras example code, with some modifications
//
// replace comments w/ CLI args where appropriate

var zarfMediaType = "application/vnd.zarf.layer.v1+tar.zst"

func (p *Packager) Publish() error {
	p.cfg.DeployOpts.PackagePath = p.cfg.PublishOpts.PackagePath
	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	registry := p.cfg.PublishOpts.RegistryURL

	name := p.cfg.Pkg.Metadata.Name
	ver := p.cfg.Pkg.Build.Version
	arch := p.cfg.Pkg.Build.Architecture
	ref := fmt.Sprintf("%s/%s:%s-%s", registry, name, ver, arch)

	message.Infof("Publishing package to %s", ref)

	pathRoot := p.tmp.Base

	ctx := context.Background()

	glob := func(path string) []string {
		paths, _ := filepath.Glob(filepath.Join(pathRoot, path))
		return paths
	}
	paths := []string{
		filepath.Join(pathRoot, "checksums.txt"),
		filepath.Join(pathRoot, "zarf.yaml"),
		filepath.Join(pathRoot, "sboms.tar.zst"),
	}
	paths = append(paths, glob("components/*.tar.zst")...)
	paths = append(paths, glob("images/*")...)

	store, err := file.New("")
	if err != nil {
		return err
	}
	defer store.Close()
	descs, err := loadFiles(ctx, store, nil, paths)
	if err != nil {
		return err
	}
	packOpts := oras.PackOptions{}
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

	// prepare push
	dst, err := remote.NewRepository(ref)
	if err != nil {
		return err
	}

	if p.cfg.PublishOpts.Insecure {
		dst.PlainHTTP = true
	}
	copyOpts := oras.DefaultCopyOptions
	if p.cfg.PublishOpts.Concurrency > copyOpts.Concurrency {
		copyOpts.Concurrency = p.cfg.PublishOpts.Concurrency
	}
	copy := func(root ocispec.Descriptor) error {
		message.Debug("%v\n", root)
		if tag := dst.Reference.Reference; tag == "" {
			err = oras.CopyGraph(ctx, store, dst, root, copyOpts.CopyGraphOptions)
		} else {
			_, err = oras.Copy(ctx, store, root.Digest.String(), dst, tag, copyOpts)
		}
		return err
	}

	// push
	root, err := pushArtifact(dst, pack, &packOpts, copy, &copyOpts.CopyGraphOptions)
	if err != nil {
		return err
	}
	message.Debugf("Pushed %v to %v", root.Digest, dst.Reference.String())
	return nil
}

type packFunc func() (ocispec.Descriptor, error)
type copyFunc func(desc ocispec.Descriptor) error

// taken from https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
func pushArtifact(dst oras.Target, pack packFunc, packOpts *oras.PackOptions, copy copyFunc, copyOpts *oras.CopyGraphOptions) (ocispec.Descriptor, error) {
	root, err := pack()
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	copyRootAttempted := false
	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
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

	// push
	if err = copy(root); err == nil {
		return root, nil
	}

	if !copyRootAttempted || root.MediaType != ocispec.MediaTypeArtifactManifest ||
		!isManifestUnsupported(err) {
		return ocispec.Descriptor{}, err
	}

	if repo, ok := dst.(*remote.Repository); ok {
		// assumes referrers API is not supported since OCI artifact
		// media type is not supported
		repo.SetReferrersCapability(false)
	}
	packOpts.PackImageManifest = true
	root, err = pack()
	if err != nil {
		return ocispec.Descriptor{}, err
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
	if err = copy(root); err != nil {
		return ocispec.Descriptor{}, err
	}
	return root, nil
}

// taken from https://github.com/oras-project/oras/blob/main/cmd/oras/file.go
func loadFiles(ctx context.Context, store *file.Store, annotations map[string]map[string]string, fileRefs []string) ([]ocispec.Descriptor, error) {
	var files []ocispec.Descriptor
	for _, filename := range fileRefs {
		// get shortest absolute path as unique name
		name := filepath.Clean(filename)
		if !filepath.IsAbs(name) {
			name = filepath.ToSlash(name)
		}

		message.Debugf("Preparing %s", name)
		file, err := store.Add(ctx, name, zarfMediaType, filename)
		if err != nil {
			return nil, err
		}
		if value, ok := annotations[filename]; ok {
			if file.Annotations == nil {
				file.Annotations = value
			} else {
				for k, v := range value {
					file.Annotations[k] = v
				}
			}
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		message.Debug("Uploading empty artifact")
	}
	return files, nil
}

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
