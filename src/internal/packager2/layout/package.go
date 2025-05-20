// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
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

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/mholt/archives"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/filters"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
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

func (p *PackageLayout) DirPath() string {
	return p.dirPath
}

// LoadFromTar unpacks the given archive (any compress/format) and loads it.
func LoadFromTar(ctx context.Context, tarPath string, opt PackageLayoutOptions) (*PackageLayout, error) {
	if opt.Filter == nil {
		opt.Filter = filters.Empty()
	}
	dirPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}

	// 1) Mount the archive as a virtual file system.
	fsys, err := archives.FileSystem(ctx, tarPath, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to open archive %q: %w", tarPath, err)
	}

	// 2) Walk every entry in the archive.
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip directories
		if d.IsDir() {
			return nil
		}
		// ensure parent dirs exist in our temp dir
		dst := filepath.Join(dirPath, path)
		if err := os.MkdirAll(filepath.Dir(dst), helpers.ReadExecuteAllWriteUser); err != nil {
			return err
		}
		// copy file contents
		in, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 3) Delegate to the existing LoadFromDir
	return LoadFromDir(ctx, dirPath, opt)
}

// LoadFromDir loads and validates a package from the given directory path.
func LoadFromDir(ctx context.Context, dirPath string, opt PackageLayoutOptions) (*PackageLayout, error) {
	if opt.Filter == nil {
		opt.Filter = filters.Empty()
	}
	b, err := os.ReadFile(filepath.Join(dirPath, ZarfYAML))
	if err != nil {
		return nil, err
	}
	pkg, err := ParseZarfPackage(ctx, b)
	if err != nil {
		return nil, err
	}
	pkg.Components, err = opt.Filter.Apply(pkg)
	if err != nil {
		return nil, err
	}
	pkgLayout := &PackageLayout{
		dirPath: dirPath,
		Pkg:     pkg,
	}
	err = validatePackageIntegrity(pkgLayout, opt.IsPartial)
	if err != nil {
		return nil, err
	}
	err = validatePackageSignature(ctx, pkgLayout, opt.PublicKeyPath, opt.SkipSignatureValidation)
	if err != nil {
		return nil, err
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

// Contains SBOM checks if a package includes an SBOM
func (p *PackageLayout) ContainsSBOM() bool {
	if !p.Pkg.IsSBOMAble() {
		return false
	}
	_, err := os.Stat(filepath.Join(p.dirPath, SBOMTar))
	return err == nil
}

// GetSBOM outputs the SBOM data from the package to the given destination path.
func (p *PackageLayout) GetSBOM(ctx context.Context, destPath string) error {
	if !p.ContainsSBOM() {
		return &NoSBOMAvailableError{pkgName: p.Pkg.Metadata.Name}
	}

	// 1) locate the sboms archive under the layout directory
	sbomArchive := filepath.Join(p.dirPath, SBOMTar)

	err := archive.Decompress(ctx, sbomArchive, destPath, archive.DecompressOpts{})
	if err != nil {
		return err
	}
	return nil
}

// GetComponentDir returns a path to the directory in the given component.
func (p *PackageLayout) GetComponentDir(ctx context.Context, destPath, componentName string, ct ComponentDir) (string, error) {
	sourcePath := filepath.Join(p.dirPath, ComponentsDir, fmt.Sprintf("%s.tar", componentName))
	_, err := os.Stat(sourcePath)
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
	defer os.RemoveAll(tmpDir)
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

func (p *PackageLayout) GetImageDir() string {
	// Use the manifest within the index.json to load the specific image we want
	return filepath.Join(p.dirPath, ImagesDir)
}

func (p *PackageLayout) Archive(ctx context.Context, dirPath string, maxPackageSize int) error {
	packageName := fmt.Sprintf("%s%s", sources.NameFromMetadata(&p.Pkg, false), sources.PkgSuffix(p.Pkg.Metadata.Uncompressed))
	tarballPath := filepath.Join(dirPath, packageName)
	err := os.Remove(tarballPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	logger.From(ctx).Info("writing package to disk", "path", tarballPath)
	files, err := os.ReadDir(p.dirPath)
	if err != nil {
		return err
	}
	var filePaths []string
	for _, file := range files {
		filePaths = append(filePaths, filepath.Join(p.dirPath, file.Name()))
	}
	err = archive.Compress(ctx, filePaths, tarballPath, archive.CompressOpts{})
	if err != nil {
		return fmt.Errorf("unable to create package: %w", err)
	}
	fi, err := os.Stat(tarballPath)
	if err != nil {
		return fmt.Errorf("unable to read the package archive: %w", err)
	}
	// Convert Megabytes to bytes.
	chunkSize := maxPackageSize * 1000 * 1000
	// If a chunk size was specified and the package is larger than the chunk size, split it into chunks.
	if maxPackageSize > 0 && fi.Size() > int64(chunkSize) {
		if fi.Size()/int64(chunkSize) > 999 {
			return fmt.Errorf("unable to split the package archive into multiple files: must be less than 1,000 files")
		}
		err := splitFile(ctx, tarballPath, chunkSize)
		if err != nil {
			return fmt.Errorf("unable to split the package archive into multiple files: %w", err)
		}
	}
	return nil
}

// Files returns a map off all the files in the package.
func (p *PackageLayout) Files() (map[string]string, error) {
	files := map[string]string{}
	err := filepath.Walk(p.dirPath, func(path string, info fs.FileInfo, err error) error {
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

func validatePackageSignature(ctx context.Context, pkgLayout *PackageLayout, publicKeyPath string, skipSignatureValidation bool) error {
	if skipSignatureValidation {
		return nil
	}

	signaturePath := filepath.Join(pkgLayout.dirPath, Signature)
	sigExist := true
	_, err := os.Stat(signaturePath)
	if err != nil {
		sigExist = false
	}
	if !sigExist && publicKeyPath == "" {
		// Nobody was expecting a signature, so we can just return
		return nil
	} else if sigExist && publicKeyPath == "" {
		return errors.New("package is signed but no key was provided")
	} else if !sigExist && publicKeyPath != "" {
		return errors.New("a key was provided but the package is not signed")
	}

	keyOptions := options.KeyOpts{KeyRef: publicKeyPath}
	cmd := &verify.VerifyBlobCmd{
		KeyOpts:    keyOptions,
		SigRef:     signaturePath,
		IgnoreSCT:  true,
		Offline:    true,
		IgnoreTlog: true,
	}
	err = cmd.Exec(ctx, filepath.Join(pkgLayout.dirPath, ZarfYAML))
	if err != nil {
		return fmt.Errorf("package signature did not match the provided key: %w", err)
	}
	return nil
}
