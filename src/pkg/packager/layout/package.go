// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
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
	Pkg     v1alpha1.ZarfPackage
	digest  string
	cache   *manifestCache
}

// Digest returns the OCI manifest digest for this package layout.
func (p *PackageLayout) Digest() string {
	return p.digest
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
	pkg, err := pkgcfg.ParseMultiDoc(ctx, b)
	if err != nil {
		return nil, err
	}
	pkg.Components, err = opts.Filter.Apply(pkg)
	if err != nil {
		return nil, err
	}
	pkgLayout := &PackageLayout{
		dirPath: dirPath,
		Pkg:     pkg,
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

// ContainsSBOM checks if a package includes an SBOM
func (p *PackageLayout) ContainsSBOM() bool {
	if !p.Pkg.IsSBOMAble() {
		return false
	}
	return !helpers.InvalidPath(filepath.Join(p.dirPath, SBOMTar))
}

// SignPackage signs the zarf package using cosign with the provided options.
// If the options do not indicate signing should be performed (no key material configured),
// this is a no-op and returns nil.
func (p *PackageLayout) SignPackage(ctx context.Context, opts signing.SignBlobOptions) (err error) {
	// This function updates in-memory state (Signed, ProvenanceFiles, VersionRequirements),
	// writes a signed zarf.yaml to a temp file, then renames the temp files into place.
	// A defer rolls back in-memory state on any error; disk state is restored best-effort
	// if a rename partially succeeds before a later rename fails.

	l := logger.From(ctx)

	// Check if signing should be performed based on the options
	// this is a no-op as there may be many different ways to sign
	// input validation should be performed in the calling function
	if !opts.ShouldSign() {
		l.Info("skipping package signing (no signing key material configured)")
		return nil
	}

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

	// Save the original signed state in case we need to rollback
	var originalSigned *bool
	if p.Pkg.Build.Signed != nil {
		val := *p.Pkg.Build.Signed
		originalSigned = &val
	}

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

	// Update in-memory state
	signed := true
	p.Pkg.Build.Signed = &signed

	// Save original fields for rollback
	originalProvenanceFiles := slices.Clone(p.Pkg.Build.ProvenanceFiles)
	originalVersionRequirements := slices.Clone(p.Pkg.Build.VersionRequirements)

	// Consolidated in-memory rollback — fires on any error exit via named return.
	defer func() {
		if err != nil {
			p.Pkg.Build.Signed = originalSigned
			p.Pkg.Build.ProvenanceFiles = originalProvenanceFiles
			p.Pkg.Build.VersionRequirements = originalVersionRequirements
		}
	}()

	// Append the bundle to the provenance files list so integrity validation can
	// dynamically exclude it from checksum enforcement.
	if !slices.Contains(p.Pkg.Build.ProvenanceFiles, Bundle) {
		p.Pkg.Build.ProvenanceFiles = append(p.Pkg.Build.ProvenanceFiles, Bundle)
		p.Pkg.Build.VersionRequirements = append(p.Pkg.Build.VersionRequirements, v1alpha1.VersionRequirement{
			Version: "v0.71.0",
			Reason:  "This package contains a bundle format signature which requires Zarf v0.71.0 or later",
		})
	}

	// Marshal package with signed:true
	b, err := goyaml.Marshal(p.Pkg)
	if err != nil {
		return fmt.Errorf("failed to marshal package for signing: %w", err)
	}

	// Write to temporary file
	if err = os.WriteFile(tmpZarfYAMLPath, b, helpers.ReadWriteUser); err != nil {
		return fmt.Errorf("failed to write temp %s: %w", ZarfYAML, err)
	}

	// Configure signing. cosign v3.1.1+ writes only the bundle when NewBundleFormat=true.
	signOpts := opts
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

	if hasBundleInfo {
		opts.TempDir = config.CommonOptions.TempDirectory
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
	opts.TempDir = config.CommonOptions.TempDirectory
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
	if p.Pkg.Build.Signed != nil {
		return *p.Pkg.Build.Signed
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

// GetSBOM outputs the SBOM data from the package to the given destination path.
func (p *PackageLayout) GetSBOM(ctx context.Context, destPath string) error {
	if !p.ContainsSBOM() {
		return &NoSBOMAvailableError{pkgName: p.Pkg.Metadata.Name}
	}

	// locate the sboms archive under the layout directory
	sbomArchive := filepath.Join(p.dirPath, SBOMTar)

	err := archive.Decompress(ctx, sbomArchive, destPath, archive.DecompressOpts{})
	if err != nil {
		return err
	}
	return nil
}

// GetDocumentation extracts documentation files from the package to the given destination path.
// If keys is empty, all documentation files are extracted.
// If keys are provided, only those specific documentation files are extracted.
func (p *PackageLayout) GetDocumentation(ctx context.Context, destPath string, keys []string) (err error) {
	l := logger.From(ctx)

	if len(p.Pkg.Documentation) == 0 {
		return fmt.Errorf("no documentation files found in package")
	}

	tarPath := filepath.Join(p.dirPath, DocumentationTar)
	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		return fmt.Errorf("documentation.tar not found in package")
	}

	keysToExtract := maps.Clone(p.Pkg.Documentation)
	if len(keys) > 0 {
		keysToExtract = make(map[string]string)
		for _, key := range keys {
			if filePath, ok := p.Pkg.Documentation[key]; ok {
				keysToExtract[key] = filePath
			} else {
				return fmt.Errorf("key %s not found in package documentation", key)
			}
		}
	}

	// Extract tar to temp directory
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	err = archive.Decompress(ctx, tarPath, tmpDir, archive.DecompressOpts{})
	if err != nil {
		return fmt.Errorf("failed to extract documentation.tar: %w", err)
	}

	if err := os.MkdirAll(destPath, helpers.ReadWriteExecuteUser); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", destPath, err)
	}

	fileNames := GetDocumentationFileNames(p.Pkg.Documentation)

	for key, file := range keysToExtract {
		docFileName := fileNames[key]

		srcPath := filepath.Join(tmpDir, docFileName)
		dstPath := filepath.Join(destPath, docFileName)
		if err := helpers.CreatePathAndCopy(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy documentation file %s: %w", file, err)
		}
	}

	l.Info("documentation successfully extracted", "path", destPath)
	return nil
}

// FormatDocumentFileName for storing the document in the package or presenting it to the user
func FormatDocumentFileName(key, file string) string {
	return fmt.Sprintf("%s-%s", key, filepath.Base(file))
}

// GetDocumentationFileNames returns a map of documentation keys to their final filenames.
// Filenames are deconflicted: if multiple keys have the same basename, they get prefixed with the key.
func GetDocumentationFileNames(documentation map[string]string) map[string]string {
	basenameCounts := make(map[string]int)
	for _, file := range documentation {
		basename := filepath.Base(file)
		basenameCounts[basename]++
	}

	result := make(map[string]string)
	for key, file := range documentation {
		basename := filepath.Base(file)
		if basenameCounts[basename] == 1 {
			result[key] = basename
		} else {
			result[key] = FormatDocumentFileName(key, file)
		}
	}
	return result
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
	return imageLayoutHasIndex(p.GetImageDirPath())
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
	if p.Pkg.Build.Architecture == "" {
		return "", errors.New("package must include a build architecture")
	}
	arch := p.Pkg.Build.Architecture

	var name string
	switch p.Pkg.Kind {
	case v1alpha1.ZarfInitConfig:
		name = fmt.Sprintf("zarf-init-%s", arch)
	case v1alpha1.ZarfPackageConfig:
		name = fmt.Sprintf("zarf-package-%s-%s", p.Pkg.Metadata.Name, arch)
	default:
		name = fmt.Sprintf("zarf-%s-%s", strings.ToLower(string(p.Pkg.Kind)), arch)
	}
	if p.Pkg.Build.Differential {
		name = fmt.Sprintf("%s-%s-differential-%s",
			name, p.Pkg.Build.DifferentialPackageVersion, p.Pkg.Metadata.Version)
	} else if p.Pkg.Metadata.Version != "" {
		name = fmt.Sprintf("%s-%s", name, p.Pkg.Metadata.Version)
	}
	if p.Pkg.Build.Flavor != "" {
		name = fmt.Sprintf("%s-%s", name, p.Pkg.Build.Flavor)
	}

	name = filepath.Base(name)

	if p.Pkg.Metadata.Uncompressed {
		return name + ".tar", nil
	}
	return name + ".tar.zst", nil
}

func validatePackageIntegrity(pkgLayout *PackageLayout, isPartial bool) error {
	_, err := os.Stat(filepath.Join(pkgLayout.dirPath, ZarfYAML))
	if err != nil {
		return err
	}
	_, err = os.Stat(filepath.Join(pkgLayout.dirPath, Checksums))
	if err != nil {
		return err
	}
	err = helpers.SHAsMatch(filepath.Join(pkgLayout.dirPath, Checksums), pkgLayout.Pkg.Metadata.AggregateChecksum)
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
		for _, f := range pkgLayout.Pkg.Build.ProvenanceFiles {
			delete(packageFiles, filepath.Join(pkgLayout.dirPath, f))
		}
	}

	b, err := os.ReadFile(filepath.Join(pkgLayout.dirPath, Checksums))
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		// If the line is empty (i.e. there is no checksum) simply skip it, this can result from a package with no images/components.
		if line == "" {
			continue
		}

		split := strings.Split(line, " ")
		if len(split) != 2 {
			return fmt.Errorf("invalid checksum line: %s", line)
		}
		sha := split[0]
		rel := split[1]
		if sha == "" || rel == "" {
			return fmt.Errorf("invalid checksum line: %s", line)
		}

		path := filepath.Join(pkgLayout.dirPath, rel)
		_, ok := packageFiles[path]
		if !ok && isPartial {
			delete(packageFiles, path)
			continue
		}
		if !ok {
			return fmt.Errorf("file %s from checksum missing in layout", rel)
		}
		err = helpers.SHAsMatch(path, sha)
		if err != nil {
			return err
		}
		delete(packageFiles, path)
	}

	if len(packageFiles) > 0 {
		filePaths := slices.Collect(maps.Keys(packageFiles))
		return fmt.Errorf("package contains additional files not present in the checksum %s", strings.Join(filePaths, ", "))
	}

	return validatePackagePaths(pkgLayout.Pkg)
}

// validatePackagePaths checks that package config fields used as filesystem
// path components do not contain path traversal sequences or separators.
func validatePackagePaths(pkg v1alpha1.ZarfPackage) error {
	if !isCleanPath(pkg.Metadata.Name) {
		return fmt.Errorf("package metadata name %q would result in an invalid path", pkg.Metadata.Name)
	}
	if !isCleanPath(pkg.Metadata.Version) {
		return fmt.Errorf("package metadata version %q would result in an invalid path", pkg.Metadata.Version)
	}
	if !isCleanPath(pkg.Build.Flavor) {
		return fmt.Errorf("package build flavor %q would result in an invalid path", pkg.Build.Flavor)
	}
	if !isCleanPath(pkg.Build.DifferentialPackageVersion) {
		return fmt.Errorf("package build differential package version %q would result in an invalid path", pkg.Build.DifferentialPackageVersion)
	}
	for _, comp := range pkg.Components {
		if !isCleanPath(comp.Name) {
			return fmt.Errorf("component name %q would result in an invalid path", comp.Name)
		}
		for _, chart := range comp.Charts {
			if !isCleanPath(chart.Name) {
				return fmt.Errorf("chart name %q in component %q would result in an invalid path", chart.Name, comp.Name)
			}
			if !isCleanPath(chart.Version) {
				return fmt.Errorf("chart version %q in component %q would result in an invalid path", chart.Version, comp.Name)
			}
		}
		for _, manifest := range comp.Manifests {
			if !isCleanPath(manifest.Name) {
				return fmt.Errorf("manifest name %q in component %q would result in an invalid path", manifest.Name, comp.Name)
			}
		}
	}
	return nil
}

// isCleanPath returns true if s is safe to embed in a file path:
// it must not be ".." and must not contain path separators.
func isCleanPath(s string) bool {
	return s != ".." && !strings.ContainsAny(s, `/\`)
}
