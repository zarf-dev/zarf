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
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// PackageLayout manages the layout for a package.
type PackageLayout struct {
	dirPath string
	Pkg     v1alpha1.ZarfPackage
}

// PackageLayoutOptions are the options used when loading a package.
type PackageLayoutOptions struct {
	PublicKeyPath           string
	SkipSignatureValidation bool
	IsPartial               bool
	Filter                  filters.ComponentFilterStrategy
}

// DirPath returns base directory of the package layout
func (p *PackageLayout) DirPath() string {
	return p.dirPath
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
	if opts.Filter == nil {
		opts.Filter = filters.Empty()
	}
	b, err := os.ReadFile(filepath.Join(dirPath, ZarfYAML))
	if err != nil {
		return nil, err
	}
	pkg, err := pkgcfg.Parse(ctx, b)
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

	if pkg.IsSigned() && !opts.SkipSignatureValidation {
		verifyOptions := utils.DefaultVerifyBlobOptions()
		verifyOptions.KeyRef = opts.PublicKeyPath

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOptions)
		if err != nil {
			return nil, err
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
func (p *PackageLayout) SignPackage(ctx context.Context, opts utils.SignBlobOptions) (err error) {
	// Note: This function:
	// 1. Updates Pkg.Build.Signed = true in memory
	// 2. Writes the updated zarf.yaml (with signed:true) to a temporary file
	// 3. Signs the temporary file
	// 4. If signing succeeds, replaces the actual zarf.yaml with the signed version
	// 5. If signing fails, reverts the in-memory state
	//
	// This ensures the zarf.yaml metadata accurately reflects the signed state and the
	// signature is valid for the zarf.yaml content that includes signed:true.

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
	tmpSignaturePath := filepath.Join(tmpDir, Signature)

	// Update in-memory state to signed:true
	signed := true
	p.Pkg.Build.Signed = &signed

	// Marshal package with signed:true
	b, err := goyaml.Marshal(p.Pkg)
	if err != nil {
		// Rollback
		p.Pkg.Build.Signed = originalSigned
		return fmt.Errorf("failed to marshal package for signing: %w", err)
	}

	// Write to temporary file
	err = os.WriteFile(tmpZarfYAMLPath, b, helpers.ReadWriteUser)
	if err != nil {
		// Rollback
		p.Pkg.Build.Signed = originalSigned
		return fmt.Errorf("failed to write temp %s: %w", ZarfYAML, err)
	}

	// Configure signing to write to temp directory
	signOpts := opts
	signOpts.OutputSignature = tmpSignaturePath

	// Check if signature already exists in actual layout and warn
	actualSignaturePath := filepath.Join(p.dirPath, Signature)
	if _, err := os.Stat(actualSignaturePath); err == nil {
		l.Warn("overwriting existing package signature", "path", actualSignaturePath)
	}

	// Perform the signing operation on the temp file
	l.Debug("signing package", "source", tmpZarfYAMLPath, "signature", tmpSignaturePath)
	_, err = utils.CosignSignBlobWithOptions(ctx, tmpZarfYAMLPath, signOpts)
	if err != nil {
		// Rollback in-memory state
		p.Pkg.Build.Signed = originalSigned
		return fmt.Errorf("failed to sign package: %w", err)
	}

	// Signing succeeded - now atomically replace the actual files

	// Move signed zarf.yaml from temp to actual location (atomic rename)
	err = os.Rename(tmpZarfYAMLPath, zarfYAMLPath)
	if err != nil {
		// This is a critical error - signing succeeded but we can't update the file
		// Keep the signed:true state as it reflects what we intended
		return fmt.Errorf("failed to update %s after signing: %w", ZarfYAML, err)
	}

	// Move signature from temp to actual location (atomic rename)
	err = os.Rename(tmpSignaturePath, actualSignaturePath)
	if err != nil {
		return fmt.Errorf("failed to move signature after signing: %w", err)
	}

	l.Info("package signed successfully", "signature", actualSignaturePath)
	return nil
}

// VerifyPackageSignature verifies the package signature
func (p *PackageLayout) VerifyPackageSignature(ctx context.Context, opts utils.VerifyBlobOptions) error {
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

	// Validate that we have a public key
	// Note: this will later be replaced when verification enhancements are made
	if opts.KeyRef == "" {
		return errors.New("package is signed but no key was provided")
	}

	// Validate that the signature exists
	signaturePath := filepath.Join(p.dirPath, Signature)
	if _, err := os.Stat(signaturePath); err != nil {
		return fmt.Errorf("signature not found: %w", err)
	}

	// Note: this is the backwards compatible behavior
	// this will change in the future
	opts.SigRef = signaturePath

	ZarfYAMLPath := filepath.Join(p.dirPath, ZarfYAML)
	return utils.CosignVerifyBlobWithOptions(ctx, ZarfYAMLPath, opts)
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
	// Remove files which are not in the checksums.
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, ZarfYAML))
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, Checksums))
	delete(packageFiles, filepath.Join(pkgLayout.dirPath, Signature))

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

	return nil
}
