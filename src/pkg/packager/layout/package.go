// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/pkg/helpers"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/checksum"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/internal/split"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// PackageLayout manages the layout for a package.
type PackageLayout struct {
	dirPath string
	pkg     api.Package
	digest  string
	cache   *manifestCache
}

// Definition returns the version-neutral package definition for this layout.
func (p *PackageLayout) Definition() api.Package {
	return p.pkg
}

// SetName updates the package metadata name.
func (p *PackageLayout) SetName(name string) {
	p.pkg.Metadata.Name = name
}

// SetAnnotations updates the package metadata annotations.
func (p *PackageLayout) SetAnnotations(annotations map[string]string) {
	p.pkg.Metadata.Annotations = maps.Clone(annotations)
}

// RemoveImages removes images and image archives from every component.
func (p *PackageLayout) RemoveImages() {
	p.pkg.RemoveImages()
}

// RemoveRepositories removes git repositories from every component.
func (p *PackageLayout) RemoveRepositories() {
	p.pkg.RemoveRepositories()
}

// SetChartNamespace sets the namespace for charts matching the component and chart names.
func (p *PackageLayout) SetChartNamespace(componentName, chartName, namespace string) {
	for i := range p.pkg.Components {
		component := &p.pkg.Components[i]
		if component.Name != componentName {
			continue
		}
		for j := range component.Charts {
			if component.Charts[j].Name == chartName {
				component.Charts[j].Namespace = namespace
			}
		}
	}
}

// Digest returns the OCI manifest digest for this package layout.
func (p *PackageLayout) Digest() string {
	return p.digest
}

// AsV1alpha1 returns the package definition as a v1alpha1 ZarfPackage.
func (p *PackageLayout) AsV1alpha1() v1alpha1.ZarfPackage {
	return convert.PackageToV1alpha1(p.pkg)
}

// AsV1beta1 returns the package definition as a v1beta1 Package.
func (p *PackageLayout) AsV1beta1() v1beta1.Package {
	return convert.PackageToV1beta1(p.pkg)
}

// OverrideNamespace overrides package namespaces when the package permits it.
func (p *PackageLayout) OverrideNamespace(namespace string) error {
	return p.pkg.OverrideNamespace(namespace)
}

// Filter applies a component filter to this layout's package definition.
func (p *PackageLayout) Filter(filter filters.ComponentFilterStrategy) error {
	definition, err := filters.Apply(p.pkg, filter)
	if err != nil {
		return err
	}
	p.pkg = definition
	return nil
}

// PackageLayoutOptions are the options used when loading a package.
type PackageLayoutOptions struct {
	// Deprecated: Use VerifyBlobOptions instead. PublicKeyPath validates the create-time signage of a package.
	PublicKeyPath string
	// VerificationStrategy specifies whether verification is enforced
	VerificationStrategy VerificationStrategy
	IsPartial            bool
	Filter               filters.ComponentFilterStrategy
	VerifyBlobOptions    *signing.VerifyBlobOptions
}

// VerificationStrategy describes a strategy for determining whether to verify a package.
type VerificationStrategy int

const (
	// VerifyIfPossible will attempt a verification, it will not error if verification
	// data is missing. But it will not stop processing if verification fails.
	VerifyIfPossible VerificationStrategy = iota
	// VerifyAlways will always attempt a verification, and will fail if the
	// verification fails.
	VerifyAlways
	// VerifyNever will skip all verification of a package.
	VerifyNever
)

// MarshalPackageDefinition returns deterministic zarf.yaml bytes for a package definition.
func MarshalPackageDefinition(definition api.Package) ([]byte, error) {
	if err := definition.Validate(); err != nil {
		return nil, fmt.Errorf("invalid package definition: %w", err)
	}
	alpha, err := goyaml.Marshal(convert.PackageToV1alpha1(definition))
	if err != nil {
		return nil, err
	}
	if definition.APIVersion != v1beta1.APIVersion {
		return alpha, nil
	}
	beta, err := goyaml.Marshal(convert.PackageToV1beta1(definition))
	if err != nil {
		return nil, err
	}
	out := append(bytes.TrimRight(alpha, "\n"), '\n')
	out = append(out, []byte("---\n")...)
	out = append(out, beta...)
	return out, nil
}

// WritePackageDefinition writes a deterministic zarf.yaml file for a package definition.
func WritePackageDefinition(path string, definition api.Package) error {
	b, err := MarshalPackageDefinition(definition)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, helpers.ReadWriteUser)
}

// ErrNoVerificationMaterial is returned when there is nothing to verify against.
// VerifyIfPossible tolerates this; all other verification errors are always fatal.
var ErrNoVerificationMaterial = errors.New("no verification material available")

// DirPath returns base directory of the package layout
func (p *PackageLayout) DirPath() string {
	return p.dirPath
}

// HasValuesSchema reports whether the package layout contains an assembled values schema file (defined or through import)
func (p *PackageLayout) HasValuesSchema() bool {
	_, err := os.Stat(filepath.Join(p.dirPath, ValuesSchema))
	return err == nil
}

// LoadFromTar unpacks the given archive (any compress/format) and loads it.
func LoadFromTar(ctx context.Context, tarPath string, opts PackageLayoutOptions) (*PackageLayout, error) {
	if opts.Filter == nil {
		opts.Filter = filters.Empty()
	}
	dirPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	// Decompress the archive
	err = archive.Decompress(ctx, tarPath, dirPath, archive.DecompressOpts{})
	if err != nil {
		return nil, err
	}

	// 3) Delegate to the existing LoadFromDir
	return LoadFromDir(ctx, dirPath, opts)
}

// LoadFromDir loads and validates a package from the given directory path.
func LoadFromDir(ctx context.Context, dirPath string, opts PackageLayoutOptions) (*PackageLayout, error) {
	l := logger.From(ctx)
	if opts.Filter == nil {
		opts.Filter = filters.Empty()
	}
	b, err := os.ReadFile(filepath.Join(dirPath, ZarfYAML))
	if err != nil {
		return nil, err
	}
	definition, err := pkgcfg.ParseMultiDoc(ctx, b)
	if err != nil {
		return nil, err
	}
	definition, err = filters.Apply(definition, opts.Filter)
	if err != nil {
		return nil, err
	}
	pkgLayout := &PackageLayout{
		dirPath: dirPath,
		pkg:     definition,
	}
	err = validatePackageIntegrity(pkgLayout, opts.IsPartial)
	if err != nil {
		return nil, err
	}

	if err := pkgLayout.computeManifest(ctx); err != nil {
		return nil, fmt.Errorf("computing OCI manifest: %w", err)
	}

	// Resolve deprecated PublicKeyPath into VerifyBlobOptions.
	// Only applies when VerifyBlobOptions is not already set,
	// ensuring the new API takes precedence over the deprecated field.
	if opts.VerifyBlobOptions == nil && opts.PublicKeyPath != "" {
		defaults := signing.DefaultVerifyBlobOptions()
		defaults.Key = opts.PublicKeyPath
		opts.VerifyBlobOptions = &defaults
	}

	if opts.VerificationStrategy != VerifyNever {
		verifyOptions := signing.DefaultVerifyBlobOptions()
		if opts.VerifyBlobOptions != nil {
			verifyOptions = *opts.VerifyBlobOptions
		}
		err = pkgLayout.VerifyPackageSignature(ctx, verifyOptions)
		if err != nil {
			// VerifyIfPossible tolerates only "nothing to verify against".
			// Tampered signatures and unsigned-with-material are always fatal.
			if opts.VerificationStrategy == VerifyIfPossible && errors.Is(err, ErrNoVerificationMaterial) {
				l.Warn("package signature not verified; continuing", "reason", err.Error())
				return pkgLayout, nil
			}
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
	}

	return pkgLayout, nil
}

// Cleanup removes any temporary directories created.
func (p *PackageLayout) Cleanup() error {
	err := os.RemoveAll(p.dirPath)
	if err != nil {
		return err
	}
	return nil
}

// NoSBOMAvailableError is returned when a user tries to access a package SBOM, but it is not available
type NoSBOMAvailableError struct {
	pkgName string
}

func (e *NoSBOMAvailableError) Error() string {
	return fmt.Sprintf("zarf package %s does not have an SBOM available", e.pkgName)
}

// ContainsSBOM checks if a package includes an SBOM.
func (p *PackageLayout) ContainsSBOM() bool {
	if !p.pkg.IsSBOMAble() {
		return false
	}
	if !UsesGranularResourceLayout(p.pkg) {
		return !helpers.InvalidPath(filepath.Join(p.dirPath, SBOMTar))
	}
	entries, err := os.ReadDir(filepath.Join(p.dirPath, SBOMResourcesDir))
	return err == nil && len(entries) > 0
}

// SignPackage signs the zarf package using cosign with the provided options.
// A nil value leaves the package unsigned; a non-nil value requests signing.
func (p *PackageLayout) SignPackage(ctx context.Context, opts *signing.SignBlobOptions) (err error) {
	if opts == nil {
		return nil
	}

	// This function updates in-memory state (Signed, ProvenanceFiles, VersionRequirements),
	// writes a signed zarf.yaml to a temp file, then renames the temp files into place.
	// A defer rolls back in-memory state on any error; disk state is restored best-effort
	// if a rename partially succeeds before a later rename fails.

	l := logger.From(ctx)

	// Validate package layout state
	if p.dirPath == "" {
		return errors.New("invalid package layout: dirPath is empty")
	}
	if info, err := os.Stat(p.dirPath); err != nil {
		return fmt.Errorf("invalid package layout directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("invalid package layout: %s is not a directory", p.dirPath)
	}

	// Verify zarf.yaml exists before signing
	zarfYAMLPath := filepath.Join(p.dirPath, ZarfYAML)
	if _, err := os.Stat(zarfYAMLPath); err != nil {
		return fmt.Errorf("cannot access %s for signing: %w", ZarfYAML, err)
	}

	originalDefinition := p.pkg

	// Create temporary directory for signing
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fmt.Errorf("failed to create temp directory for signing: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	tmpZarfYAMLPath := filepath.Join(tmpDir, ZarfYAML)
	tmpBundlePath := filepath.Join(tmpDir, Bundle)

	definition := p.pkg
	signed := true
	definition.Build.Signed = &signed
	if !slices.Contains(definition.Build.ProvenanceFiles, Bundle) {
		definition.Build.ProvenanceFiles = append(definition.Build.ProvenanceFiles, Bundle)
	}
	requirement := api.VersionRequirement{
		Version: "v0.71.0",
		Reason:  "This package contains a bundle format signature which requires Zarf v0.71.0 or later",
	}
	definition.Build.VersionRequirements = append(definition.Build.VersionRequirements, requirement)
	p.pkg = definition

	// Consolidated in-memory rollback — fires on any error exit via named return.
	defer func() {
		if err != nil {
			p.pkg = originalDefinition
		}
	}()

	// Marshal package with signed:true
	b, err := MarshalPackageDefinition(definition)
	if err != nil {
		return fmt.Errorf("failed to marshal package for signing: %w", err)
	}

	// Write to temporary file
	if err = os.WriteFile(tmpZarfYAMLPath, b, helpers.ReadWriteUser); err != nil {
		return fmt.Errorf("failed to write temp %s: %w", ZarfYAML, err)
	}

	// Configure signing. cosign v3.1.1+ writes only the bundle when NewBundleFormat=true.
	signOpts := *opts
	signOpts.NewBundleFormat = true
	actualBundlePath := filepath.Join(p.dirPath, Bundle)
	signOpts.BundlePath = actualBundlePath
	if err = signOpts.CheckOverwrite(ctx); err != nil {
		return err
	}

	signOpts.BundlePath = tmpBundlePath

	// Perform the signing operation on the temp file
	l.Debug("signing package", "source", tmpZarfYAMLPath, "bundle", tmpBundlePath)
	if _, err = signing.CosignSignBlobWithOptions(ctx, tmpZarfYAMLPath, signOpts); err != nil {
		return fmt.Errorf("failed to sign package: %w", err)
	}

	// Read original zarf.yaml bytes for disk rollback if a subsequent rename fails.
	originalZarfYAMLBytes, err := os.ReadFile(zarfYAMLPath)
	if err != nil {
		return fmt.Errorf("failed to read %s before rename: %w", ZarfYAML, err)
	}

	// Atomically replace the actual files. On partial failure, restore disk state.
	if err = os.Rename(tmpZarfYAMLPath, zarfYAMLPath); err != nil {
		return fmt.Errorf("failed to update %s after signing: %w", ZarfYAML, err)
	}

	if err = os.Rename(tmpBundlePath, actualBundlePath); err != nil {
		if writeErr := os.WriteFile(zarfYAMLPath, originalZarfYAMLBytes, helpers.ReadWriteUser); writeErr != nil {
			l.Warn("failed to restore original zarf.yaml after bundle rename failure", "error", writeErr)
		}
		return fmt.Errorf("failed to move bundle after signing: %w", err)
	}

	// Remove any legacy zarf.yaml.sig left from a previous sign operation.
	// The bundle supersedes it; leaving it in place would be misleading.
	legacySignaturePath := filepath.Join(p.dirPath, Signature)
	if rmErr := os.Remove(legacySignaturePath); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		l.Warn("failed to remove legacy signature file", "path", legacySignaturePath, "error", rmErr)
	}

	if info, bundleErr := signing.ReadBundleInfo(actualBundlePath); bundleErr == nil {
		if info.Identity != "" {
			l.Info("keyless signed package", "identity", info.Identity, "issuer", info.Issuer)
		}
	} else {
		l.Debug("could not read bundle info after signing", "error", bundleErr)
	}

	if err := p.computeManifest(ctx); err != nil {
		return fmt.Errorf("recomputing OCI manifest after signing: %w", err)
	}

	l.Info("package signed successfully")
	return nil
}

// VerifyPackageSignature verifies the package signature
func (p *PackageLayout) VerifyPackageSignature(ctx context.Context, opts signing.VerifyBlobOptions) error {
	l := logger.From(ctx)
	l.Debug("verifying package signature")

	// Validate package layout state
	if p.dirPath == "" {
		return errors.New("invalid package layout: dirPath is empty")
	}
	if info, err := os.Stat(p.dirPath); err != nil {
		return fmt.Errorf("invalid package layout directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("invalid package layout: %s is not a directory", p.dirPath)
	}

	// Sync the deprecated KeyRef alias before computing hasKey so callers using
	// only KeyRef are not rejected for missing material. CosignVerifyBlobWithOptions
	// emits the deprecation warning when invoked.
	if opts.Key == "" && opts.KeyRef != "" { //nolint:staticcheck // intentional read of deprecated alias for migration sync
		opts.Key = opts.KeyRef //nolint:staticcheck // intentional read of deprecated alias for migration sync
	}

	hasKey := opts.Key != ""
	hasKeylessIdentity := opts.CertVerify.CertIdentity != "" || opts.CertVerify.CertIdentityRegexp != ""
	hasCert := opts.CertVerify.Cert != ""
	hasVerificationMaterial := hasKey || hasKeylessIdentity || hasCert

	// Handle the case where the package is not signed
	if !p.IsSigned() {
		if hasVerificationMaterial {
			// Providing material implies expecting a signature — always fatal.
			return errors.New("verification material was provided but the package is not signed")
		}
		return fmt.Errorf("package is not signed - verification cannot be performed: %w", ErrNoVerificationMaterial)
	}

	// Check for bundle format signature (preferred). Parse it once for both method
	// detection (fast-fail below) and the verify path.
	bundlePath := filepath.Join(p.dirPath, Bundle)
	bundleInfo, bundleErr := signing.ReadBundleInfo(bundlePath)
	hasBundleInfo := bundleErr == nil

	// Early validation: fail fast with a method-specific message before cosign emits a generic error.
	if hasBundleInfo {
		switch bundleInfo.Method {
		case signing.SigningMethodKeyless:
			if !hasKeylessIdentity && !hasCert {
				return fmt.Errorf("package was signed with keyless method; provide --certificate-identity + --certificate-oidc-issuer to verify: %w", ErrNoVerificationMaterial)
			}
		case signing.SigningMethodKey:
			if !hasKey && !hasCert {
				return fmt.Errorf("package was signed with a key; provide --key to verify: %w", ErrNoVerificationMaterial)
			}
		}
	}

	if !hasVerificationMaterial {
		return fmt.Errorf("package is signed but no verification material was provided (--key, --certificate-identity + --certificate-oidc-issuer): %w", ErrNoVerificationMaterial)
	}

	// Preserve a caller-provided staging directory while retaining Zarf's default.
	if opts.TempDir == "" {
		opts.TempDir = config.CommonOptions.TempDirectory
	}

	if hasBundleInfo {
		opts.BundlePath = bundlePath
		// Auto-enable UseSignedTimestamps when the bundle contains timestamps.
		// The bundle was signed with a TSA; using those timestamps is required to
		// verify the signature after the short-lived Fulcio cert expires.
		if bundleInfo.HasTSATimestamps && !opts.CommonVerifyOptions.UseSignedTimestamps {
			l.Debug("bundle contains TSA timestamps; enabling signed-timestamp verification automatically")
			opts.CommonVerifyOptions.UseSignedTimestamps = true
		}
		ZarfYAMLPath := filepath.Join(p.dirPath, ZarfYAML)
		return signing.CosignVerifyBlobWithOptions(ctx, ZarfYAMLPath, opts)
	}
	if !errors.Is(bundleErr, os.ErrNotExist) {
		return fmt.Errorf("error checking bundle signature: %w", bundleErr)
	}

	// Bundle doesn't exist, check for legacy signature format
	signaturePath := filepath.Join(p.dirPath, Signature)
	_, sigStatErr := os.Stat(signaturePath)
	if sigStatErr != nil {
		if errors.Is(sigStatErr, os.ErrNotExist) {
			return fmt.Errorf("signature not found: neither bundle nor legacy signature exists")
		}
		return fmt.Errorf("error checking legacy signature: %w", sigStatErr)
	}

	// Legacy signatures don't carry a certificate chain, so keyless identity
	// verification has nothing to match against. Fail fast with a clear message
	// rather than letting cosign emit a generic key/cert/bundle error.
	if hasKeylessIdentity {
		return errors.New("keyless verification requires bundle-format signatures, but this package has only a legacy .sig. Ask the publisher to re-sign with bundle format, or verify with --key")
	}

	// Legacy signature found
	l.Warn("bundle format signature not found: legacy signature is being deprecated.")
	opts.Signature = signaturePath

	opts.CommonVerifyOptions.NewBundleFormat = false
	ZarfYAMLPath := filepath.Join(p.dirPath, ZarfYAML)
	return signing.CosignVerifyBlobWithOptions(ctx, ZarfYAMLPath, opts)
}

// IsSigned returns true if the package is signed.
// It first checks the package metadata (Build.Signed), then falls back to
// checking for the presence of a signature file for backward compatibility.
func (p *PackageLayout) IsSigned() bool {
	// Check metadata first (authoritative source)
	if p.pkg.Build.Signed != nil {
		return *p.pkg.Build.Signed
	}

	// Backward compatibility: check for signature file existence
	// This handles packages created before the Build.Signed field was added
	if p.dirPath != "" {
		if _, err := os.Stat(filepath.Join(p.dirPath, Signature)); err == nil {
			return true
		}
	}

	return false
}

// GetSBOM outputs selected SBOM data from the package to the given destination path.
// An empty key set outputs every SBOM.
func (p *PackageLayout) GetSBOM(ctx context.Context, destPath string, keys []string) (err error) {
	if !p.ContainsSBOM() {
		return &NoSBOMAvailableError{pkgName: p.Definition().Metadata.Name}
	}
	if !UsesGranularResourceLayout(p.pkg) {
		if len(keys) > 0 {
			return fmt.Errorf("SBOM selection by key requires package API version v1beta1+")
		}
		return archive.Decompress(ctx, filepath.Join(p.dirPath, SBOMTar), destPath, archive.DecompressOpts{})
	}

	resources, err := p.sbomResources(keys)
	if err != nil {
		return err
	}
	outputPaths, err := sbomResourceOutputPaths(resources)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destPath, helpers.ReadWriteExecuteUser); err != nil {
		return fmt.Errorf("creating SBOM output directory: %w", err)
	}
	destinationRoot, err := os.OpenRoot(destPath)
	if err != nil {
		return fmt.Errorf("opening SBOM output directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, destinationRoot.Close())
	}()
	for index, resource := range resources {
		if err := copyResourceFile(resource.sourcePath, destinationRoot, outputPaths[index]); err != nil {
			return err
		}
	}
	return nil
}

type sbomResource struct {
	kind         SBOMResourceKind
	identifier   string
	resourcePath string
	sourcePath   string
}

func (p *PackageLayout) sbomResources(keys []string) ([]sbomResource, error) {
	var requested map[string]struct{}
	var imageReferences map[string]struct{}
	if len(keys) > 0 {
		requested = make(map[string]struct{}, len(keys))
		for _, key := range keys {
			requested[key] = struct{}{}
		}
		imageReferences = SBOMImageReferenceSet(p.pkg)
	}
	foundRequested := make(map[string]struct{}, len(requested))
	resourceRoot := filepath.Join(p.dirPath, SBOMResourcesDir)
	resources := []sbomResource{}
	err := filepath.WalkDir(resourceRoot, func(sourcePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		resourcePath, err := filepath.Rel(p.dirPath, sourcePath)
		if err != nil {
			return fmt.Errorf("calculating SBOM resource path: %w", err)
		}
		resourcePath = filepath.ToSlash(resourcePath)
		kind, identifier, ok := ParseSBOMResourcePath(resourcePath)
		if !ok {
			return fmt.Errorf("invalid SBOM resource path %q", resourcePath)
		}
		if len(requested) > 0 {
			selected := false
			for key := range requested {
				if !SBOMKeyMatchesResource(kind, identifier, key, imageReferences) {
					continue
				}
				foundRequested[key] = struct{}{}
				selected = true
			}
			if !selected {
				return nil
			}
		}
		resources = append(resources, sbomResource{
			kind:         kind,
			identifier:   identifier,
			resourcePath: resourcePath,
			sourcePath:   sourcePath,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if _, found := foundRequested[key]; !found {
			return nil, fmt.Errorf("SBOM key %q not found in package", key)
		}
	}
	return resources, nil
}

func sbomResourceOutputPaths(resources []sbomResource) ([]string, error) {
	outputPaths := make([]string, len(resources))
	resourcePathsByOutputPath := make(map[string]string, len(resources))
	for index, resource := range resources {
		outputPath, err := sbomResourceOutputPath(resource)
		if err != nil {
			return nil, err
		}
		if conflictingPath, exists := resourcePathsByOutputPath[outputPath]; exists && conflictingPath != resource.resourcePath {
			return nil, fmt.Errorf("SBOM resources %q and %q both normalize to %q", conflictingPath, resource.resourcePath, filepath.ToSlash(outputPath))
		}
		resourcePathsByOutputPath[outputPath] = resource.resourcePath
		outputPaths[index] = outputPath
	}
	return outputPaths, nil
}

func sbomResourceOutputPath(resource sbomResource) (string, error) {
	switch resource.kind {
	case SBOMResourceImage:
		return filepath.Join("images", NormalizeSBOMFilename(resource.identifier+".json")), nil
	case SBOMResourceComponent:
		relativePath, found := strings.CutPrefix(resource.resourcePath, SBOMResourcesDir+"/")
		if !found {
			return "", fmt.Errorf("SBOM resource path %q is outside resource root", resource.resourcePath)
		}
		return filepath.FromSlash(relativePath), nil
	default:
		return "", fmt.Errorf("unsupported SBOM resource kind %q", resource.kind)
	}
}

func copyResourceFile(sourcePath string, destinationRoot *os.Root, outputPath string) (err error) {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stating package resource: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("package resource %q is not a regular file", sourcePath)
	}
	if dir := filepath.Dir(outputPath); dir != "." {
		if err := destinationRoot.MkdirAll(dir, helpers.ReadWriteExecuteUser); err != nil {
			return fmt.Errorf("creating SBOM output directory: %w", err)
		}
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("opening package resource: %w", err)
	}
	defer func() {
		err = errors.Join(err, source.Close())
	}()
	destination, err := destinationRoot.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, helpers.ReadAllWriteUser)
	if err != nil {
		return fmt.Errorf("opening SBOM output file: %w", err)
	}
	defer func() {
		err = errors.Join(err, destination.Close())
	}()
	_, err = io.Copy(destination, source)
	return err
}

// GetDocumentation extracts documentation files from the package to the given destination path.
//
// Deprecated: Use GetAttachments instead.
func (p *PackageLayout) GetDocumentation(ctx context.Context, destPath string, keys []string) error {
	return p.GetAttachments(ctx, destPath, keys)
}

// GetAttachments extracts attachment files (documentation in v1alpha1) to the given destination path.
// If keys is empty, all attachments are extracted.
// If keys are provided, only those specific attachments are extracted.
func (p *PackageLayout) GetAttachments(ctx context.Context, destPath string, keys []string) (err error) {
	l := logger.From(ctx)
	if len(p.pkg.Attachments) == 0 {
		return fmt.Errorf("no attachments found in package")
	}

	fileNames, err := GetAttachmentFileNames(p.pkg.Attachments, keys)
	if err != nil {
		return err
	}

	sourceDir := filepath.Join(p.dirPath, AttachmentsDir)
	if !UsesGranularResourceLayout(p.pkg) {
		tarPath := filepath.Join(p.dirPath, DocumentationTar)
		if _, err := os.Stat(tarPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("documentation.tar not found in package")
			}
			return err
		}
		sourceDir, err = utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer func() {
			err = errors.Join(err, os.RemoveAll(sourceDir))
		}()
		if err := archive.Decompress(ctx, tarPath, sourceDir, archive.DecompressOpts{}); err != nil {
			return fmt.Errorf("failed to extract documentation.tar: %w", err)
		}
	}

	if err := os.MkdirAll(destPath, helpers.ReadWriteExecuteUser); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", destPath, err)
	}

	sourceRoot, err := os.OpenRoot(sourceDir)
	if err != nil {
		return fmt.Errorf("opening package attachment root: %w", err)
	}
	defer func() {
		err = errors.Join(err, sourceRoot.Close())
	}()

	destinationRoot, err := os.OpenRoot(destPath)
	if err != nil {
		return fmt.Errorf("failed to open attachment output root: %w", err)
	}
	defer func() {
		err = errors.Join(err, destinationRoot.Close())
	}()
	for key, attachmentFileName := range fileNames {
		if err := copyAttachmentFile(sourceRoot, destinationRoot, attachmentFileName); err != nil {
			return fmt.Errorf("failed to copy attachment file %s: %w", p.pkg.Attachments[key], err)
		}
	}

	l.Info("attachments successfully extracted", "path", destPath)
	return nil
}

func copyAttachmentFile(sourceRoot, destinationRoot *os.Root, name string) error {
	info, err := sourceRoot.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("attachment file %q is not a regular file", name)
	}

	source, err := sourceRoot.Open(name)
	if err != nil {
		return err
	}

	destination, err := destinationRoot.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, helpers.ReadAllWriteUser)
	if err != nil {
		return errors.Join(err, source.Close())
	}

	_, err = io.Copy(destination, source)
	return errors.Join(err, source.Close(), destination.Close())
}

// FormatAttachmentFileName returns the stored filename for an attachment whose basename collides with another attachment.
func FormatAttachmentFileName(key, file string) string {
	return fmt.Sprintf("%s-%s", key, filepath.Base(file))
}

// FormatDocumentFileName returns the stored filename for a documentation file whose basename collides with another document.
//
// Deprecated: Use FormatAttachmentFileName.
func FormatDocumentFileName(key, file string) string {
	return FormatAttachmentFileName(key, file)
}

// GetDocumentationFileNames returns validated stored filenames for the requested documentation keys.
//
// Deprecated: Use GetAttachmentFileNames.
func GetDocumentationFileNames(documentation map[string]string, keys []string) (map[string]string, error) {
	return GetAttachmentFileNames(documentation, keys)
}

// GetAttachmentFileNames returns validated stored filenames for the requested attachment keys.
// Filenames are deconflicted before selection: if multiple keys have the same basename, they are prefixed with the key.
func GetAttachmentFileNames(attachments map[string]string, keys []string) (map[string]string, error) {
	basenameCounts := make(map[string]int, len(attachments))
	basenames := make(map[string]string, len(attachments))
	for key, file := range attachments {
		if !isCleanAttachmentKey(key) {
			return nil, fmt.Errorf("attachment key %q would result in an invalid path", key)
		}
		if strings.Contains(key, " ") {
			return nil, fmt.Errorf("attachment key %q must not contain spaces", key)
		}
		basename := filepath.Base(file)
		if !isCleanAttachmentFileName(basename) {
			return nil, fmt.Errorf("attachment file %q would result in an invalid path", file)
		}
		if strings.Contains(basename, " ") {
			return nil, fmt.Errorf("attachment file %q produces final filename %q, which must not contain spaces", file, basename)
		}
		basenames[key] = basename
		basenameCounts[basename]++
	}

	fileNames := make(map[string]string, len(attachments))
	for key, file := range attachments {
		basename := basenames[key]
		if basenameCounts[basename] == 1 {
			fileNames[key] = basename
		} else {
			fileNames[key] = FormatAttachmentFileName(key, file)
		}
	}
	if len(keys) == 0 {
		return fileNames, nil
	}

	selected := make(map[string]string, len(keys))
	for _, key := range keys {
		fileName, found := fileNames[key]
		if !found {
			return nil, fmt.Errorf("attachment key %s not found in package", key)
		}
		selected[key] = fileName
	}
	return selected, nil
}

// GetComponentDir returns a path to the directory in the given component.
func (p *PackageLayout) GetComponentDir(ctx context.Context, destPath, componentName string, ct ComponentDir) (_ string, err error) {
	sourcePath := filepath.Join(p.dirPath, ComponentsDir, fmt.Sprintf("%s.tar", componentName))
	_, err = os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("component %s does not exist in package: %w", componentName, err)
	}
	if err != nil {
		return "", err
	}
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()
	err = archive.Decompress(ctx, sourcePath, tmpDir, archive.DecompressOpts{})
	if err != nil {
		return "", err
	}
	compPath := filepath.Join(tmpDir, componentName, string(ct))
	_, err = os.Stat(compPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("component %s could not access a %s directory: %w", componentName, ct, err)
	}
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(destPath, string(ct))
	err = os.Rename(compPath, outPath)
	if err != nil {
		return "", err
	}
	return outPath, nil
}

// GetImageDirPath returns the path to the images directory
func (p *PackageLayout) GetImageDirPath() string {
	// Use the manifest within the index.json to load the specific image we want
	return filepath.Join(p.dirPath, ImagesDir)
}

// HasImageIndex reports whether the package layout has a multi-platform image
func (p *PackageLayout) HasImageIndex() (bool, error) {
	return HasImageIndex(p.GetImageDirPath())
}

// Archive creates a tarball from the package layout and returns the path to that tarball
func (p *PackageLayout) Archive(ctx context.Context, dirPath string, maxPackageSize int) (string, error) {
	filename, err := p.FileName()
	if err != nil {
		return "", err
	}
	tarballPath := filepath.Join(dirPath, filename)
	err = os.Remove(tarballPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	logger.From(ctx).Info("writing package to disk", "path", tarballPath)
	files, err := os.ReadDir(p.dirPath)
	if err != nil {
		return "", err
	}
	var filePaths []string
	for _, file := range files {
		filePaths = append(filePaths, filepath.Join(p.dirPath, file.Name()))
	}
	err = archive.Compress(ctx, filePaths, tarballPath, archive.CompressOpts{})
	if err != nil {
		return "", fmt.Errorf("unable to create package: %w", err)
	}
	fi, err := os.Stat(tarballPath)
	if err != nil {
		return "", fmt.Errorf("unable to read the package archive: %w", err)
	}
	// Convert Megabytes to bytes.
	chunkSize := maxPackageSize * 1000 * 1000
	// If a chunk size was specified and the package is larger than the chunk size, split it into chunks.
	if maxPackageSize > 0 && fi.Size() > int64(chunkSize) {
		if fi.Size()/int64(chunkSize) > 999 {
			return "", fmt.Errorf("unable to split the package archive into multiple files: must be less than 1,000 files")
		}
		var err error
		tarballPath, err = split.SplitFile(ctx, tarballPath, chunkSize)
		if err != nil {
			return "", fmt.Errorf("unable to split the package archive into multiple files: %w", err)
		}
	}
	return tarballPath, nil
}

// Files returns a map of all the files in the package.
func (p *PackageLayout) Files() (map[string]string, error) {
	files := map[string]string{}
	err := filepath.Walk(p.dirPath, func(path string, info fs.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(p.dirPath, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		files[path] = name
		return err
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// FileName returns the name of the Zarf package should have when exported to the file system
func (p *PackageLayout) FileName() (string, error) {
	pkg := p.pkg
	if pkg.Build.Architecture == "" {
		return "", errors.New("package must include a build architecture")
	}
	arch := pkg.Build.Architecture

	var name string
	switch pkg.Kind {
	case api.ZarfInitConfig:
		name = fmt.Sprintf("zarf-init-%s", arch)
	case api.ZarfPackageConfig:
		name = fmt.Sprintf("zarf-package-%s-%s", pkg.Metadata.Name, arch)
	default:
		name = fmt.Sprintf("zarf-%s-%s", strings.ToLower(string(pkg.Kind)), arch)
	}
	if pkg.Build.Differential != nil {
		name = fmt.Sprintf("%s-%s-differential-%s",
			name, pkg.Build.Differential.FromVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		name = fmt.Sprintf("%s-%s", name, pkg.Metadata.Version)
	}
	if pkg.Build.Flavor != "" {
		name = fmt.Sprintf("%s-%s", name, pkg.Build.Flavor)
	}

	name = filepath.Base(name)

	if pkg.Metadata.Uncompressed {
		return name + ".tar", nil
	}
	return name + ".tar.zst", nil
}

func validatePackageIntegrity(pkgLayout *PackageLayout, isPartial bool) error {
	pkg := pkgLayout.pkg
	_, err := os.Stat(filepath.Join(pkgLayout.dirPath, ZarfYAML))
	if err != nil {
		return err
	}
	_, err = os.Stat(filepath.Join(pkgLayout.dirPath, Checksums))
	if err != nil {
		return err
	}
	err = checksum.VerifyFile(filepath.Join(pkgLayout.dirPath, Checksums), api.ChecksumSHA256, pkg.Build.AggregateChecksum)
	if err != nil {
		return err
	}

	packageFiles, err := pkgLayout.Files()
	if err != nil {
		return err
	}
	// zarf.yaml is the root of trust and is always excluded from checksums.
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, ZarfYAML))
	// Hardcoded exclusions for backward compatibility with packages that predate
	// the ProvenanceFiles field. These can be removed once all supported
	// package versions include ProvenanceFiles.
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, Checksums))
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, Signature))
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, Bundle))
	// Remove provenance files declared in the signed zarf.yaml.
	// This enables forward compatibility — new files added by future CLI versions
	// are excluded from the strict check without requiring code changes.
	if pkgLayout.IsSigned() {
		for _, f := range pkg.Build.ProvenanceFiles {
			delete(packageFiles, filepath.Join(pkgLayout.dirPath, f))
		}
	}

	checksums, err := os.ReadFile(filepath.Join(pkgLayout.dirPath, Checksums))
	if err != nil {
		return err
	}
	checksumEntries, err := checksum.Parse(checksums)
	if err != nil {
		return err
	}
	for _, entry := range checksumEntries {
		path := filepath.Join(pkgLayout.dirPath, entry.Path)
		_, ok := packageFiles[path]
		if !ok && isPartial {
			delete(packageFiles, path)
			continue
		}
		if !ok {
			return fmt.Errorf("file %s from checksum missing in layout", entry.Path)
		}
		err = checksum.VerifyFile(path, api.ChecksumSHA256, entry.Digest)
		if err != nil {
			return err
		}
		delete(packageFiles, path)
	}

	if len(packageFiles) > 0 {
		filePaths := slices.Collect(maps.Keys(packageFiles))
		return fmt.Errorf("package contains additional files not present in the checksum %s", strings.Join(filePaths, ", "))
	}

	return ValidatePackagePaths(pkg)
}

// ValidatePackagePaths checks that package config fields used as filesystem
// path components do not contain path traversal sequences or separators.
func ValidatePackagePaths(pkg api.Package) error {
	if !isCleanPath(pkg.Metadata.Name) {
		return fmt.Errorf("package metadata name %q would result in an invalid path", pkg.Metadata.Name)
	}
	if !isCleanPath(pkg.Metadata.Version) {
		return fmt.Errorf("package metadata version %q would result in an invalid path", pkg.Metadata.Version)
	}
	if !isCleanPath(pkg.Build.Flavor) {
		return fmt.Errorf("package build flavor %q would result in an invalid path", pkg.Build.Flavor)
	}
	if pkg.Build.Differential != nil && !isCleanPath(pkg.Build.Differential.FromVersion) {
		return fmt.Errorf("package build differential package version %q would result in an invalid path", pkg.Build.Differential.FromVersion)
	}
	for _, comp := range pkg.Components {
		if !isCleanPath(comp.Name) {
			return fmt.Errorf("component name %q would result in an invalid path", comp.Name)
		}
		for _, chart := range comp.Charts {
			if !isCleanPath(chart.Name) {
				return fmt.Errorf("chart name %q in component %q would result in an invalid path", chart.Name, comp.Name)
			}
			if !isCleanPath(chart.LegacyVersion) {
				return fmt.Errorf("chart version %q in component %q would result in an invalid path", chart.LegacyVersion, comp.Name)
			}
		}
		for _, manifest := range comp.Manifests {
			if !isCleanPath(manifest.Name) {
				return fmt.Errorf("manifest name %q in component %q would result in an invalid path", manifest.Name, comp.Name)
			}
		}
	}
	if _, err := GetAttachmentFileNames(pkg.Attachments, nil); err != nil {
		return err
	}
	return nil
}

// isCleanPath returns true if s is safe to embed in a file path:
// it must not be ".." and must not contain path separators.
func isCleanPath(s string) bool {
	return s != ".." && !strings.ContainsAny(s, `/\`)
}

func isCleanAttachmentKey(key string) bool {
	return key != "" && key != "." && isCleanPath(key)
}

func isCleanAttachmentFileName(fileName string) bool {
	return fileName != "" && fileName != "." && isCleanPath(fileName)
}
