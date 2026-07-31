// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package component publishes and pulls standalone v1beta1 Zarf component configs.
package component

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/xeipuuv/gojsonschema"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/schema"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	orasremote "oras.land/oras-go/v2/registry/remote"
)

const (
	artifactType             = "application/vnd.zarf.component.v1"
	componentLayerMediaType  = "application/vnd.zarf.component.layer.v1.blob"
	componentConfigFileName  = "component.yaml"
	defaultPublishRetryDelay = 500 * time.Millisecond
	defaultPublishMaxDelay   = 8 * time.Second
)

// ConfigName is the canonical file name for a pulled component config.
const ConfigName = componentConfigFileName

// PublishOptions configures component publishing.
type PublishOptions struct {
	OCIConcurrency int
	Retries        int
	Flavor         string
	Tag            string
	CachePath      string
	RemoteOptions  types.RemoteOptions
}

// PullOptions configures component pulling.
type PullOptions struct {
	OCIConcurrency int
	CachePath      string
	RemoteOptions  types.RemoteOptions
}

// Publish reads a v1beta1 ZarfComponentConfig and publishes it as an OCI artifact.
func Publish(ctx context.Context, componentFile string, dst registry.Reference, opts PublishOptions) (registry.Reference, error) {
	if err := dst.ValidateRegistry(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}
	if err := dst.ValidateRepository(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid repository: %w", err)
	}
	if dst.Reference != "" {
		return registry.Reference{}, fmt.Errorf("destination reference must not include a tag or digest")
	}
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return registry.Reference{}, fmt.Errorf("retries cannot be negative")
		}
		opts.Retries = zoci.DefaultRetries
	}

	buildDir, cleanup, err := makeTempDir(opts.CachePath)
	if err != nil {
		return registry.Reference{}, err
	}
	defer func() {
		if err := cleanup(); err != nil {
			logger.From(ctx).Warn("unable to remove component publish directory", "path", buildDir, "error", err)
		}
	}()

	cfg, err := prepareBuildDir(ctx, componentFile, buildDir, opts.Flavor)
	if err != nil {
		return registry.Reference{}, err
	}

	tag := opts.Tag
	if tag == "" {
		tag = cfg.Metadata.Version
	}
	if tag == "" {
		return registry.Reference{}, fmt.Errorf("component %q must specify metadata.version or publish tag", cfg.Metadata.Name)
	}

	published := registry.Reference{
		Registry:   dst.Registry,
		Repository: path.Join(dst.Repository, cfg.Metadata.Name),
		Reference:  tag,
	}
	if err := published.Validate(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid component reference: %w", err)
	}

	store, err := file.New(buildDir)
	if err != nil {
		return registry.Reference{}, err
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.From(ctx).Warn("unable to close component file store", "path", buildDir, "error", err)
		}
	}()
	store.SkipUnpack = true

	layers, err := addBuildLayers(ctx, store, buildDir)
	if err != nil {
		return registry.Reference{}, err
	}
	root, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, artifactType, oras.PackManifestOptions{
		Layers: layers,
		ManifestAnnotations: map[string]string{
			ocispec.AnnotationTitle: cfg.Metadata.Name,
		},
	})
	if err != nil {
		return registry.Reference{}, fmt.Errorf("packing component artifact: %w", err)
	}
	if err := store.Tag(ctx, root, tag); err != nil {
		return registry.Reference{}, fmt.Errorf("tagging component artifact: %w", err)
	}

	repo, err := newRepository(published, opts.RemoteOptions)
	if err != nil {
		return registry.Reference{}, err
	}
	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = concurrencyOrDefault(opts.OCIConcurrency)

	err = retry.Do(
		func() error {
			_, err := oras.Copy(ctx, store, tag, repo, published.Reference, copyOpts)
			return err
		},
		retry.Attempts(uint(opts.Retries)),
		retry.Delay(defaultPublishRetryDelay),
		retry.MaxDelay(defaultPublishMaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
	)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("publishing component artifact: %w", err)
	}
	return published, nil
}

// Pull downloads a published component artifact and returns its component.yaml path.
func Pull(ctx context.Context, ref registry.Reference, opts PullOptions) (localPath string, cleanup func() error, err error) {
	if err := ref.Validate(); err != nil {
		return "", nil, fmt.Errorf("invalid component reference: %w", err)
	}
	if ref.Reference == "" {
		return "", nil, fmt.Errorf("remote component reference must include a tag or digest")
	}

	dir, cleanup, err := makeTempDir(opts.CachePath)
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, cleanup())
			cleanup = nil
		}
	}()

	repo, err := newRepository(ref, opts.RemoteOptions)
	if err != nil {
		return "", nil, err
	}
	store, err := file.New(dir)
	if err != nil {
		return "", nil, err
	}
	defer func() {
		err = errors.Join(err, store.Close())
	}()

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = concurrencyOrDefault(opts.OCIConcurrency)
	if _, err := oras.Copy(ctx, repo, ref.Reference, store, ref.Reference, copyOpts); err != nil {
		return "", nil, fmt.Errorf("pulling component artifact: %w", err)
	}

	localPath = filepath.Join(dir, componentConfigFileName)
	info, err := os.Stat(localPath)
	if err != nil {
		return "", nil, fmt.Errorf("pulled component artifact missing %s: %w", componentConfigFileName, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("pulled %s is a directory", componentConfigFileName)
	}
	return localPath, cleanup, nil
}

func newRepository(ref registry.Reference, remoteOpts types.RemoteOptions) (*orasremote.Repository, error) {
	repoRef := registry.Reference{Registry: ref.Registry, Repository: ref.Repository}
	repo, err := orasremote.NewRepository(repoRef.String())
	if err != nil {
		return nil, err
	}
	repo.PlainHTTP = remoteOpts.PlainHTTP
	return repo, nil
}

func prepareBuildDir(ctx context.Context, componentFile, buildDir, flavor string) (v1beta1.ComponentConfig, error) {
	root, err := loadComponentConfig(componentFile, flavor)
	if err != nil {
		return v1beta1.ComponentConfig{}, err
	}
	root.config.PublishData.ZarfVersion = config.CLIVersion
	if err := writeComponentConfig(filepath.Join(buildDir, componentConfigFileName), root.config); err != nil {
		return v1beta1.ComponentConfig{}, err
	}

	collector := resourceCollector{
		buildDir: buildDir,
		seen:     map[string]string{},
		configs:  map[string]bool{},
	}
	if err := collector.collect(ctx, root, ".", flavor); err != nil {
		return v1beta1.ComponentConfig{}, err
	}
	return root.config, nil
}

type loadedConfig struct {
	config v1beta1.ComponentConfig
	path   string
	dir    string
}

// FIXME: p is a bad name
func loadComponentConfig(p, flavor string) (loadedConfig, error) {
	info, err := os.Stat(p)
	if err != nil {
		return loadedConfig{}, fmt.Errorf("unable to access component config %q: %w", p, err)
	}
	if info.IsDir() {
		return loadedConfig{}, fmt.Errorf("component config path %q is a directory", p)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return loadedConfig{}, err
	}

	var untyped any
	if err := goyaml.Unmarshal(b, &untyped); err != nil {
		return loadedConfig{}, fmt.Errorf("unable to parse component config %q: %w", p, err)
	}
	if err := validateComponentSchema(untyped); err != nil {
		return loadedConfig{}, fmt.Errorf("component config %q failed schema validation: %w", p, err)
	}

	var cfg v1beta1.ComponentConfig
	if err := goyaml.Unmarshal(b, &cfg); err != nil {
		return loadedConfig{}, fmt.Errorf("unable to parse component config %q: %w", p, err)
	}
	if cfg.APIVersion != v1beta1.APIVersion {
		return loadedConfig{}, fmt.Errorf("component config %q apiVersion must be %s", p, v1beta1.APIVersion)
	}
	if cfg.Kind != v1beta1.ZarfComponentConfig {
		return loadedConfig{}, fmt.Errorf("component config %q kind must be %s", p, v1beta1.ZarfComponentConfig)
	}
	if cfg.Component.Selector.Flavor != "" && cfg.Component.Selector.Flavor != flavor {
		return loadedConfig{}, fmt.Errorf("component %q requires flavor %q", cfg.Metadata.Name, cfg.Component.Selector.Flavor)
	}
	if err := validateComponentEquivalent(cfg); err != nil {
		return loadedConfig{}, fmt.Errorf("component config %q failed component validation: %w", p, err)
	}
	if err := validateImports(cfg.Component.Import); err != nil {
		return loadedConfig{}, err
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return loadedConfig{}, err
	}
	return loadedConfig{config: cfg, path: abs, dir: filepath.Dir(abs)}, nil
}

func validateComponentSchema(doc any) error {
	result, err := gojsonschema.Validate(gojsonschema.NewBytesLoader(schema.GetV1Beta1ComponentSchema()), gojsonschema.NewGoLoader(doc))
	if err != nil {
		return err
	}
	if result.Valid() {
		return nil
	}
	parts := make([]string, 0, len(result.Errors()))
	for _, schemaErr := range result.Errors() {
		parts = append(parts, schemaErr.String())
	}
	return errors.New(strings.Join(parts, "; "))
}

func validateComponentEquivalent(cfg v1beta1.ComponentConfig) error {
	pkg := v1beta1.Package{
		Metadata: v1beta1.PackageMetadata{Name: cfg.Metadata.Name, Version: cfg.Metadata.Version},
		Components: []v1beta1.Component{{
			Name:          cfg.Metadata.Name,
			ComponentSpec: cfg.Component,
		}},
	}
	return internalv1beta1.ValidatePackage(pkg)
}

func validateImports(imp v1beta1.ComponentImport) error {
	for _, l := range imp.Local {
		if l.Path == "" {
			return fmt.Errorf("import entry is missing a path")
		}
		if filepath.IsAbs(l.Path) {
			return fmt.Errorf("import path %q cannot be absolute", l.Path)
		}
	}
	for _, r := range imp.Remote {
		if r.URL == "" {
			return fmt.Errorf("remote import entry is missing a url")
		}
		if !strings.HasPrefix(r.URL, helpers.OCIURLPrefix) {
			return fmt.Errorf("remote import url %q must start with %s", r.URL, helpers.OCIURLPrefix)
		}
		trimmed := strings.TrimPrefix(r.URL, helpers.OCIURLPrefix)
		ref, err := registry.ParseReference(trimmed)
		if err != nil {
			return fmt.Errorf("remote import url %q is invalid: %w", r.URL, err)
		}
		if ref.Reference == "" {
			return fmt.Errorf("remote import url %q must include a tag or digest", r.URL)
		}
	}
	return nil
}

type resourceCollector struct {
	buildDir string
	seen     map[string]string
	configs  map[string]bool
}

func (c *resourceCollector) collect(ctx context.Context, cfg loadedConfig, dstDirRel, flavor string) error {
	if c.configs[cfg.path] {
		return fmt.Errorf("component config %s imported in cycle", filepath.ToSlash(cfg.path))
	}
	c.configs[cfg.path] = true
	defer delete(c.configs, cfg.path)

	for _, imp := range cfg.config.Component.Import.Local {
		srcPath := filepath.Clean(filepath.Join(cfg.dir, imp.Path))
		loaded, err := loadComponentConfig(srcPath, flavor)
		if err != nil {
			return err
		}
		dstRel, err := containedRel(filepath.Join(dstDirRel, imp.Path))
		if err != nil {
			return fmt.Errorf("import path %q cannot be included: %w", imp.Path, err)
		}
		if err := c.copyPath(loaded.path, dstRel); err != nil {
			return err
		}
		if err := c.collect(ctx, loaded, filepath.Dir(dstRel), flavor); err != nil {
			return err
		}
	}

	for _, rel := range localResourcePaths(cfg.config) {
		dstRel, err := containedRel(filepath.Join(dstDirRel, rel))
		if err != nil {
			return fmt.Errorf("resource path %q cannot be included: %w", rel, err)
		}
		srcPath := rel
		if !filepath.IsAbs(srcPath) {
			srcPath = filepath.Join(cfg.dir, rel)
		}
		if err := c.copyPath(srcPath, dstRel); err != nil {
			return err
		}
	}
	return nil
}

func localResourcePaths(cfg v1beta1.ComponentConfig) []string {
	var paths []string
	add := func(p string) {
		if p == "" || helpers.IsURL(p) {
			return
		}
		paths = append(paths, p)
	}
	for _, f := range cfg.Component.Files {
		add(f.Source)
	}
	// FIXME: make sure image archives works how we would expect
	for _, a := range cfg.Component.ImageArchives {
		add(a.Path)
	}
	for _, chart := range cfg.Component.Charts {
		if chart.Local != nil {
			add(chart.Local.Path)
		}
		for _, vf := range chart.ValuesFiles {
			add(vf.Path)
		}
	}
	for _, manifest := range cfg.Component.Manifests {
		for _, p := range manifest.Files {
			add(p)
		}
		if manifest.Kustomize != nil {
			for _, p := range manifest.Kustomize.Files {
				add(p)
			}
		}
	}
	for _, p := range cfg.Values.Files {
		add(p)
	}
	add(cfg.Values.Schema)
	return paths
}

func (c *resourceCollector) copyPath(src, dstRel string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("unable to access local resource %q: %w", src, err)
	}
	if existing, ok := c.seen[dstRel]; ok {
		if existing != src {
			return fmt.Errorf("multiple resources map to %q", dstRel)
		}
		return nil
	}
	c.seen[dstRel] = src
	dst := filepath.Join(c.buildDir, dstRel)
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	return errors.Join(copyErr, closeErr)
}

func containedRel(p string) (string, error) {
	clean := filepath.Clean(p)
	if clean == "." || clean == string(filepath.Separator) || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes artifact root")
	}
	return clean, nil
}

func writeComponentConfig(path string, cfg v1beta1.ComponentConfig) error {
	b, err := goyaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func addBuildLayers(ctx context.Context, store *file.Store, buildDir string) ([]ocispec.Descriptor, error) {
	var rels []string
	if err := filepath.WalkDir(buildDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(buildDir, p)
		if err != nil {
			return err
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, fmt.Errorf("component artifact has no files")
	}
	slices.Sort(rels)
	layers := make([]ocispec.Descriptor, 0, len(rels))
	for _, rel := range rels {
		desc, err := store.Add(ctx, rel, componentLayerMediaType, filepath.Join(buildDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		layers = append(layers, desc)
	}
	return layers, nil
}

func makeTempDir(parent string) (string, func() error, error) {
	if parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", nil, err
		}
	}
	dir, err := os.MkdirTemp(parent, "zarf-component-*")
	if err != nil {
		return "", nil, err
	}
	return dir, func() error { return os.RemoveAll(dir) }, nil
}

func concurrencyOrDefault(concurrency int) int {
	if concurrency <= 0 {
		return zoci.DefaultConcurrency
	}
	return concurrency
}
