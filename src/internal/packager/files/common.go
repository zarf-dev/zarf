// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package files contains functions for interacting with, managing and deploying Zarf files.
package files

import (
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/variables"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// FileCfg is a config object for packing and processing Zarf files.
type FileCfg struct {
	File           *types.ZarfFile
	FilePrefix     string
	Component      *types.ZarfComponent
	ComponentPaths types.ComponentPaths
	ValueTemplate  *variables.Values
}

// PackFiles packs all component files into a package
func (f *FileCfg) PackFile() error {
	message.Debugf("Loading file from %q", f.File.Source)

	matrixFiles := f.getMatrixFileMap()

	for _, mf := range matrixFiles {
		dst, _ := mf.getFilePath()

		if helpers.IsURL(mf.File.Source) {
			if err := utils.DownloadToFile(mf.File.Source, dst, f.Component.DeprecatedCosignKeyPath); err != nil {
				return fmt.Errorf(lang.ErrDownloading, mf.File.Source, err.Error())
			}
		} else {
			if err := utils.CreatePathAndCopy(mf.File.Source, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", mf.File.Source, err)
			}
		}

		if err := mf.finalizeFile(); err != nil {
			return err
		}
	}

	return nil
}

// PackSkeletonFiles packs all applicable component files into a skeleton package
func (f *FileCfg) PackSkeletonFile() error {
	message.Debugf("Loading file from %q", f.File.Source)

	matrixFiles := f.getMatrixFileMap()

	for _, mf := range matrixFiles {
		dst, rel := mf.getFilePath()

		if !helpers.IsURL(mf.File.Source) {
			if err := utils.CreatePathAndCopy(mf.File.Source, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", mf.File.Source, err)
			}
			f.File.Source = rel
			if err := mf.finalizeFile(); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetSBOMPaths returns paths to all component files for SBOMing
func (f *FileCfg) GetSBOMPaths() []string {
	paths := []string{}

	matrixFiles := f.getMatrixFileMap()

	for _, mf := range matrixFiles {
		dst, _ := mf.getFilePath()
		paths = append(paths, dst)
	}

	return paths
}

// ProcessFiles moves files onto the host of the machine performing the deployment.
func (f *FileCfg) ProcessFile() error {
	pkgLocation := f.ComponentPaths.Files

	// spinner.Updatef("Loading %s", file.Target)

	if f.File.Matrix != nil {
		prefix := fmt.Sprintf("%s-%s-%s", f.FilePrefix, runtime.GOOS, runtime.GOARCH)

		matrixFiles := f.getMatrixFileMap()
		if mFile, ok := matrixFiles[prefix]; ok {
			f.File = mFile.File
		} else {
			return fmt.Errorf("the %q operating system on the %q platform is not supported by this package", runtime.GOOS, runtime.GOARCH)
		}
	}

	_, fileLocation := f.getFilePath()
	if utils.InvalidPath(fileLocation) {
		fileLocation = filepath.Join(pkgLocation, f.FilePrefix)
	}

	// If a shasum is specified check it again on deployment as well
	if f.File.Shasum != "" {
		// spinner.Updatef("Validating SHASUM for %s", file.Target)
		if shasum, _ := utils.GetCryptoHashFromFile(fileLocation, crypto.SHA256); shasum != f.File.Shasum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", f.File.Source, f.File.Shasum, shasum)
		}
	}

	// Replace temp target directory and home directory
	f.File.Target = strings.Replace(f.File.Target, "###ZARF_TEMP###", f.ComponentPaths.Package, 1)
	f.File.Target = config.GetAbsHomePath(f.File.Target)

	fileList := []string{}
	if utils.IsDir(fileLocation) {
		files, _ := utils.RecursiveFileList(fileLocation, nil, false)
		fileList = append(fileList, files...)
	} else {
		fileList = append(fileList, fileLocation)
	}

	for _, subFile := range fileList {
		// Check if the file looks like a text file
		isText, err := utils.IsTextFile(subFile)
		if err != nil {
			message.Debugf("unable to determine if file %s is a text file: %s", subFile, err)
		}

		// If the file is a text file, template it
		if isText {
			// spinner.Updatef("Templating %s", file.Target)
			if err := f.ValueTemplate.Apply(*f.Component, subFile, true); err != nil {
				return fmt.Errorf("unable to template file %s: %w", subFile, err)
			}
		}
	}

	// Copy the file to the destination
	// spinner.Updatef("Saving %s", file.Target)
	err := utils.CreatePathAndCopy(fileLocation, f.File.Target)
	if err != nil {
		return fmt.Errorf("unable to copy file %s to %s: %w", fileLocation, f.File.Target, err)
	}

	// Loop over all symlinks and create them
	for _, link := range f.File.Symlinks {
		// spinner.Updatef("Adding symlink %s->%s", link, file.Target)
		// Try to remove the filepath if it exists
		_ = os.RemoveAll(link)
		// Make sure the parent directory exists
		_ = utils.CreateFilePath(link)
		// Create the symlink
		err := os.Symlink(f.File.Target, link)
		if err != nil {
			return fmt.Errorf("unable to create symlink %s->%s: %w", link, f.File.Target, err)
		}
	}

	// Cleanup now to reduce disk pressure
	_ = os.RemoveAll(fileLocation)

	// spinner.Success()

	return nil
}

func (f *FileCfg) getMatrixFileMap() map[string]*FileCfg {
	matrixFiles := make(map[string]*FileCfg)
	if f.File.Matrix != nil {
		matrixValue := reflect.ValueOf(*f.File.Matrix)
		for fieldIdx := 0; fieldIdx < matrixValue.NumField(); fieldIdx++ {
			prefix := fmt.Sprintf("%s-%s", f.FilePrefix, helpers.GetJSONTagName(matrixValue, fieldIdx))
			if options, ok := matrixValue.Field(fieldIdx).Interface().(*types.ZarfFileOptions); ok && options != nil {
				r := *f.File
				if options.Shasum != "" {
					r.Shasum = options.Shasum
				}
				if options.Source != "" {
					r.Source = options.Source
				}
				if options.Target != "" {
					r.Target = options.Target
				}
				if len(options.Symlinks) > 0 {
					r.Symlinks = options.Symlinks
				}

				mFileCfg := FileCfg{
					File:           &r,
					FilePrefix:     prefix,
					ComponentPaths: f.ComponentPaths,
					ValueTemplate:  f.ValueTemplate,
				}

				matrixFiles[prefix] = &mFileCfg
			}
		}
	} else {
		matrixFiles[f.FilePrefix] = f
	}

	return matrixFiles
}

func (f *FileCfg) getFilePath() (string, string) {
	rel := filepath.Join(types.FilesFolder, f.FilePrefix, filepath.Base(f.File.Target))
	dst := filepath.Join(f.ComponentPaths.Base, rel)
	return dst, rel
}

func (f *FileCfg) finalizeFile() error {
	dst, _ := f.getFilePath()

	// Abort packaging on invalid shasum (if one is specified).
	if f.File.Shasum != "" {
		if actualShasum, _ := utils.GetCryptoHashFromFile(dst, crypto.SHA256); actualShasum != f.File.Shasum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", f.File.Source, f.File.Shasum, actualShasum)
		}
	}

	if f.File.Executable || utils.IsDir(dst) {
		_ = os.Chmod(dst, 0700)
	} else {
		_ = os.Chmod(dst, 0600)
	}
	return nil
}
