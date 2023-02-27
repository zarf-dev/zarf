// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// ZarfLayerMediaType<Extension> is the media type for Zarf layers.
const (
	ZarfLayerMediaTypeTarZstd = "application/vnd.zarf.layer.v1.tar+zstd"
	ZarfLayerMediaTypeTarGzip = "application/vnd.zarf.layer.v1.tar+gzip"
	ZarfLayerMediaTypeYaml    = "application/vnd.zarf.layer.v1.yaml"
	ZarfLayerMediaTypeJSON    = "application/vnd.zarf.layer.v1.json"
	ZarfLayerMediaTypeTxt     = "application/vnd.zarf.layer.v1.txt"
	ZarfLayerMediaTypeUnknown = "application/vnd.zarf.layer.v1.unknown"
)

// parseZarfLayerMediaType returns the Zarf layer media type for the given filename.
func (p *Packager) parseZarfLayerMediaType(filename string) string {
	// since we are controlling the filenames, we can just use the extension
	switch filepath.Ext(filename) {
	case ".zst":
		return ZarfLayerMediaTypeTarZstd
	case ".gz":
		return ZarfLayerMediaTypeTarGzip
	case ".yaml":
		return ZarfLayerMediaTypeYaml
	case ".json":
		return ZarfLayerMediaTypeJSON
	case ".txt":
		return ZarfLayerMediaTypeTxt
	default:
		return ZarfLayerMediaTypeUnknown
	}
}

// orasCtxWithScopes returns a context with the given scopes.
//
// This is needed for pushing to Docker Hub.
func (p *Packager) orasCtxWithScopes(ref registry.Reference) context.Context {
	// For pushing to Docker Hub, we need to set the scope to the repository with pull+push actions, otherwise a 401 is returned
	scopes := []string{
		fmt.Sprintf("repository:%s:pull,push", ref.Repository),
	}
	return auth.WithScopes(context.Background(), scopes...)
}

// orasAuthClient returns an auth client for the given reference.
//
// The credentials are pulled using Docker's default credential store.
func (p *Packager) orasAuthClient(ref registry.Reference) (*auth.Client, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return &auth.Client{}, err
	}
	if !cfg.ContainsAuth() {
		return &auth.Client{}, errors.New("no docker config file found, run 'zarf tools registry login --help'")
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = ref.Registry
	if key == "registry-1.docker.io" || key == "docker.io" {
		// Docker stores its credentials under the following key, otherwise credentials use the registry URL
		key = "https://index.docker.io/v1/"
	}

	authConf, err := configs[0].GetCredentialsStore(key).Get(key)
	if err != nil {
		return &auth.Client{}, fmt.Errorf("unable to get credentials for %s: %w", key, err)
	}

	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: zarfconfig.CommonOptions.Insecure,
	}
	// TODO:(@RAZZLE) https://github.com/oras-project/oras/blob/e8bc5acd9b7be47f2f9f387af6a963b14ae49eda/cmd/oras/internal/option/remote.go#L183

	return &auth.Client{
		Credential: auth.StaticCredential(ref.Registry, cred),
		Cache:      auth.NewCache(),
		// Gitlab auth fails if ForceAttemptOAuth2 is set to true
		// ForceAttemptOAuth2: true,
		Client: &http.Client{
			Transport: transport,
		},
	}, nil
}

// PullOCIZarfPackage downloads a Zarf package w/ the given reference to the specified output directory.
//
// If the current implementation causes memory issues, we can
// refactor to use oras.Copy which uses a memory buffer.
func (p *Packager) pullOCIZarfPackage(ref registry.Reference, out string) error {
	mSpinner := message.NewMultiSpinner().Start()
	defer mSpinner.Stop()
	_ = os.Mkdir(out, 0755)
	repo, ctx, err := p.orasRemote(ref)
	if err != nil {
		return err
	}

	first30last30 := func(s string) string {
		if len(s) > 60 {
			return s[0:27] + "..." + s[len(s)-26:]
		}
		return s
	}
	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = p.cfg.DeployOpts.CopyOptions.Concurrency
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		rows := mSpinner.GetContent()
		title := desc.Annotations[ocispec.AnnotationTitle]
		var format string
		if title != "" {
			format = fmt.Sprintf("%s %s", desc.Digest.Hex()[:12], first30last30(title))
		} else {
			format = fmt.Sprintf("%s [%s]", desc.Digest.Hex()[:12], desc.MediaType)
		}
		rows = append(rows, message.NewMultiSpinnerRow(format))
		mSpinner.Update(rows)
		return nil
	}
	copyOpts.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
		rows := mSpinner.GetContent()
		for idx, row := range rows {
			if strings.HasPrefix(row.Text, desc.Digest.Hex()[:12]) {
				mSpinner.RowSuccess(idx)
				break
			}
		}
		return nil
	}
	copyOpts.PostCopy = copyOpts.OnCopySkipped

	dst, err := file.New(out)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = oras.Copy(ctx, repo,ref.Reference, dst, ref.Reference, copyOpts)
	if err != nil {
		return err
	}

	return nil
}

func (p *Packager) orasRemote(ref registry.Reference) (*remote.Repository, context.Context, error) {
	// patch docker.io to registry-1.docker.io
	if ref.Registry == "docker.io" {
		ref.Registry = "registry-1.docker.io"
	}
	ctx := p.orasCtxWithScopes(ref)
	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return &remote.Repository{}, ctx, err
	}
	repo.PlainHTTP = zarfconfig.CommonOptions.Insecure
	authClient, err := p.orasAuthClient(ref)
	if err != nil {
		return &remote.Repository{}, ctx, err
	}
	repo.Client = authClient
	return repo, ctx, nil
}
