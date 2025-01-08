// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// CreateOptions are the options for creating a skeleton package.
type CreateOptions struct {
	Flavor             string
	RegistryOverrides  map[string]string
	SigningKeyPath     string
	SigningKeyPassword string
	SetVariables       map[string]string
}

// CreateSkeleton creates a skeleton package and returns the path to the created package.
func CreateSkeleton(ctx context.Context, packagePath string, opt CreateOptions) (string, error) {
	b, err := os.ReadFile(filepath.Join(packagePath, ZarfYAML))
	if err != nil {
		return "", err
	}
	var pkg v1alpha1.ZarfPackage
	err = goyaml.Unmarshal(b, &pkg)
	if err != nil {
		return "", err
	}
	buildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", err
	}

	pkg.Metadata.Architecture = config.GetArch()

	pkg, err = resolveImports(ctx, pkg, packagePath, pkg.Metadata.Architecture, opt.Flavor, map[string]interface{}{})
	if err != nil {
		return "", err
	}

	pkg.Metadata.Architecture = zoci.SkeletonArch

	err = validate(pkg, packagePath, opt.SetVariables)
	if err != nil {
		return "", err
	}

	for _, component := range pkg.Components {
		err := assembleComponent(component, packagePath, buildPath)
		if err != nil {
			return "", err
		}
	}

	checksumContent, checksumSha, err := getChecksum(buildPath)
	if err != nil {
		return "", err
	}
	checksumPath := filepath.Join(buildPath, Checksums)
	err = os.WriteFile(checksumPath, []byte(checksumContent), helpers.ReadWriteUser)
	if err != nil {
		return "", err
	}
	pkg.Metadata.AggregateChecksum = checksumSha

	pkg = recordPackageMetadata(pkg, opt.Flavor, opt.RegistryOverrides)

	b, err = goyaml.Marshal(pkg)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filepath.Join(buildPath, ZarfYAML), b, helpers.ReadWriteUser)
	if err != nil {
		return "", err
	}

	err = signPackage(buildPath, opt.SigningKeyPath, opt.SigningKeyPassword)
	if err != nil {
		return "", err
	}

	return buildPath, nil
}

func validate(pkg v1alpha1.ZarfPackage, packagePath string, setVariables map[string]string) error {
	err := lint.ValidatePackage(pkg)
	if err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}
	findings, err := lint.ValidatePackageSchemaAtPath(packagePath, setVariables)
	if err != nil {
		return fmt.Errorf("unable to check schema: %w", err)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		BaseDir:     packagePath,
		PackageName: pkg.Metadata.Name,
		Findings:    findings,
	}
}

func assembleComponent(component v1alpha1.ZarfComponent, packagePath, buildPath string) error {
	tmpBuildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpBuildPath)
	compBuildPath := filepath.Join(tmpBuildPath, component.Name)
	err = os.MkdirAll(compBuildPath, 0o700)
	if err != nil {
		return err
	}

	for chartIdx, chart := range component.Charts {
		if chart.LocalPath != "" {
			rel := filepath.Join(string(ChartsComponentDir), fmt.Sprintf("%s-%d", chart.Name, chartIdx))
			dst := filepath.Join(compBuildPath, rel)

			err := helpers.CreatePathAndCopy(filepath.Join(packagePath, chart.LocalPath), dst)
			if err != nil {
				return err
			}

			component.Charts[chartIdx].LocalPath = rel
		}

		for valuesIdx, path := range chart.ValuesFiles {
			if helpers.IsURL(path) {
				continue
			}

			rel := fmt.Sprintf("%s-%d", helm.StandardName(string(ValuesComponentDir), chart), valuesIdx)
			component.Charts[chartIdx].ValuesFiles[valuesIdx] = rel

			if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, path), filepath.Join(compBuildPath, rel)); err != nil {
				return fmt.Errorf("unable to copy chart values file %s: %w", path, err)
			}
		}
	}

	for filesIdx, file := range component.Files {
		if helpers.IsURL(file.Source) {
			continue
		}

		rel := filepath.Join(string(FilesComponentDir), strconv.Itoa(filesIdx), filepath.Base(file.Target))
		dst := filepath.Join(compBuildPath, rel)
		destinationDir := filepath.Dir(dst)

		if file.ExtractPath != "" {
			if err := archiver.Extract(filepath.Join(packagePath, file.Source), file.ExtractPath, destinationDir); err != nil {
				return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, file.Source, err.Error())
			}

			// Make sure dst reflects the actual file or directory.
			updatedExtractedFileOrDir := filepath.Join(destinationDir, file.ExtractPath)
			if updatedExtractedFileOrDir != dst {
				if err := os.Rename(updatedExtractedFileOrDir, dst); err != nil {
					return fmt.Errorf(lang.ErrWritingFile, dst, err)
				}
			}
		} else {
			if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, file.Source), dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", file.Source, err)
			}
		}

		// Change the source to the new relative source directory (any remote files will have been skipped above)
		component.Files[filesIdx].Source = rel

		// Remove the extractPath from a skeleton since it will already extract it
		component.Files[filesIdx].ExtractPath = ""

		// Abort packaging on invalid shasum (if one is specified).
		if file.Shasum != "" {
			if err := helpers.SHAsMatch(dst, file.Shasum); err != nil {
				return err
			}
		}

		if file.Executable || helpers.IsDir(dst) {
			err = os.Chmod(dst, helpers.ReadWriteExecuteUser)
			if err != nil {
				return err
			}
		} else {
			err = os.Chmod(dst, helpers.ReadWriteUser)
			if err != nil {
				return err
			}
		}
	}

	for dataIdx, data := range component.DataInjections {
		rel := filepath.Join(string(DataComponentDir), strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
		dst := filepath.Join(compBuildPath, rel)

		if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, data.Source), dst); err != nil {
			return fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
		}

		component.DataInjections[dataIdx].Source = rel
	}

	// Iterate over all manifests.
	for manifestIdx, manifest := range component.Manifests {
		for fileIdx, path := range manifest.Files {
			rel := filepath.Join(string(ManifestsComponentDir), fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
			dst := filepath.Join(compBuildPath, rel)

			// Copy manifests without any processing.
			if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, path), dst); err != nil {
				return fmt.Errorf("unable to copy manifest %s: %w", path, err)
			}

			component.Manifests[manifestIdx].Files[fileIdx] = rel
		}

		for kustomizeIdx, path := range manifest.Kustomizations {
			// Generate manifests from kustomizations and place in the package.
			kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
			rel := filepath.Join(string(ManifestsComponentDir), kname)
			dst := filepath.Join(compBuildPath, rel)

			if err := kustomize.Build(filepath.Join(packagePath, path), dst, manifest.KustomizeAllowAnyDirectory); err != nil {
				return fmt.Errorf("unable to build kustomization %s: %w", path, err)
			}
		}

		// remove kustomizations
		component.Manifests[manifestIdx].Kustomizations = nil
	}

	// Write the tar component.
	entries, err := os.ReadDir(compBuildPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	err = os.MkdirAll(filepath.Join(compBuildPath, "temp"), 0o700)
	if err != nil {
		return err
	}
	tarPath := filepath.Join(buildPath, "components", fmt.Sprintf("%s.tar", component.Name))
	err = os.MkdirAll(filepath.Join(buildPath, "components"), 0o700)
	if err != nil {
		return err
	}
	err = createReproducibleTarballFromDir(compBuildPath, component.Name, tarPath)
	if err != nil {
		return err
	}
	return nil
}

func recordPackageMetadata(pkg v1alpha1.ZarfPackage, flavor string, registryOverrides map[string]string) v1alpha1.ZarfPackage {
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		pkg.Build.User = os.Getenv("USERNAME")
	} else {
		pkg.Build.User = os.Getenv("USER")
	}

	// Record the hostname of the package creation terminal.
	// The error here is ignored because the hostname is not critical to the package creation.
	hostname, _ := os.Hostname()
	pkg.Build.Terminal = hostname

	if pkg.IsInitConfig() && pkg.Metadata.Version == "" {
		pkg.Metadata.Version = config.CLIVersion
	}

	pkg.Build.Architecture = pkg.Metadata.Architecture

	// Record the Zarf Version the CLI was built with.
	pkg.Build.Version = config.CLIVersion

	// Record the time of package creation.
	pkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the flavor of Zarf used to build this package (if any).
	pkg.Build.Flavor = flavor

	pkg.Build.RegistryOverrides = registryOverrides

	return pkg
}

func getChecksum(dirPath string) (string, string, error) {
	checksumData := []string{}
	err := filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		if rel == ZarfYAML || rel == Checksums {
			return nil
		}
		sum, err := helpers.GetSHA256OfFile(path)
		if err != nil {
			return err
		}
		checksumData = append(checksumData, fmt.Sprintf("%s %s", sum, filepath.ToSlash(rel)))
		return nil
	})
	if err != nil {
		return "", "", err
	}
	slices.Sort(checksumData)

	checksumContent := strings.Join(checksumData, "\n") + "\n"
	sha := sha256.Sum256([]byte(checksumContent))
	return checksumContent, hex.EncodeToString(sha[:]), nil
}

func signPackage(dirPath, signingKeyPath, signingKeyPassword string) error {
	if signingKeyPath == "" {
		return nil
	}
	passFunc := func(_ bool) ([]byte, error) {
		return []byte(signingKeyPassword), nil
	}
	keyOpts := options.KeyOpts{
		KeyRef:   signingKeyPath,
		PassFunc: passFunc,
	}
	rootOpts := &options.RootOptions{
		Verbose: false,
		Timeout: options.DefaultTimeout,
	}
	_, err := sign.SignBlobCmd(
		rootOpts,
		keyOpts,
		filepath.Join(dirPath, ZarfYAML),
		true,
		filepath.Join(dirPath, Signature),
		"",
		false)
	if err != nil {
		return err
	}
	return nil
}

func createReproducibleTarballFromDir(dirPath, dirPrefix, tarballPath string) error {
	tb, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %w", err)
	}
	defer tb.Close()

	tw := tar.NewWriter(tb)
	defer tw.Close()

	// Walk through the directory and process each file
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		link := ""
		if info.Mode().Type() == os.ModeSymlink {
			link, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("error reading symlink: %w", err)
			}
		}

		// Create a new header
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("error creating tar header: %w", err)
		}

		// Strip non-deterministic header data
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""

		// When run on windows the header mode will set all permission octals to the same value as the first octal.
		// A file created with 0o700 will return 0o777 when read back. This discrepancy causes differences between packages
		// created on Windows and Linux.
		// https://medium.com/@MichalPristas/go-and-file-perms-on-windows-3c944d55dd44
		// To mitigate this difference we zero all but the last permission octal when writing files to the tar. Making sure
		// that when unpackaged files from packages created on Windows and Linux will have the same permissions.
		// The &^ operator called AND NOT sets the bits to 0 in the left hand if the right hand bits are 1.
		// https://medium.com/learning-the-go-programming-language/bit-hacking-with-go-e0acee258827
		header.Mode = header.Mode &^ 0o077

		// Ensure the header's name is correctly set relative to the base directory
		name, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}
		name = filepath.Join(dirPrefix, name)
		name = filepath.ToSlash(name)
		header.Name = name

		// Write the header to the tarball
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing header: %w", err)
		}

		// If it's a file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("error opening file: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("error writing file to tarball: %w", err)
			}
		}

		return nil
	})
}
