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
	file           *types.ZarfFile
	filePrefix     string
	component      *types.ZarfComponent
	componentPaths types.ComponentPaths
	valueTemplate  *variables.Values
}

func New(file *types.ZarfFile, filePrefix string, component *types.ZarfComponent, componentPaths types.ComponentPaths) *FileCfg {
	return &FileCfg{
		file:           file,
		filePrefix:     filePrefix,
		component:      component,
		componentPaths: componentPaths,
	}
}

func (f *FileCfg) WithValues(valueTemplate *variables.Values) *FileCfg {
	f.valueTemplate = valueTemplate
	return f
}

// PackFile packs all component files into a package
func (f *FileCfg) PackFile() error {
	message.Debugf("Loading file from %q", f.file.Source)

	matrixFiles := f.getMatrixFileMap()

	for _, mf := range matrixFiles {
		dst, _ := mf.getFilePath()

		if helpers.IsURL(mf.file.Source) {
			if err := utils.DownloadToFile(mf.file.Source, dst, f.component.DeprecatedCosignKeyPath); err != nil {
				return fmt.Errorf(lang.ErrDownloading, mf.file.Source, err.Error())
			}
		} else {
			if err := utils.CreatePathAndCopy(mf.file.Source, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", mf.file.Source, err)
			}
		}

		if err := mf.finalizeFile(); err != nil {
			return err
		}
	}

	return nil
}

// PackSkeletonFile packs all applicable component files into a skeleton package
func (f *FileCfg) PackSkeletonFile() error {
	message.Debugf("Loading file from %q", f.file.Source)

	matrixFiles := f.getMatrixFileMap()

	for _, mf := range matrixFiles {
		dst, rel := mf.getFilePath()

		if !helpers.IsURL(mf.file.Source) {
			if err := utils.CreatePathAndCopy(mf.file.Source, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", mf.file.Source, err)
			}
			f.file.Source = rel
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

// ProcessFile moves files onto the host of the machine performing the deployment.
func (f *FileCfg) ProcessFile() error {
	// spinner.Updatef("Loading %s", file.Target)

	if f.file.Matrix != nil {
		prefix := fmt.Sprintf("%s-%s-%s", f.filePrefix, runtime.GOOS, runtime.GOARCH)

		matrixFiles := f.getMatrixFileMap()
		if mFile, ok := matrixFiles[prefix]; ok {
			f.file = mFile.file
		} else {
			return fmt.Errorf("the %q operating system on the %q platform is not supported by this package", runtime.GOOS, runtime.GOARCH)
		}
	}

	filePkgPath, _ := f.getFilePath()
	if utils.InvalidPath(filePkgPath) {
		filePkgPath = filepath.Join(f.componentPaths.Files, f.filePrefix)
	}

	// If a shasum is specified check it again on deployment as well
	if f.file.Shasum != "" {
		// spinner.Updatef("Validating SHASUM for %s", file.Target)
		if shasum, _ := utils.GetCryptoHashFromFile(filePkgPath, crypto.SHA256); shasum != f.file.Shasum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", f.file.Source, f.file.Shasum, shasum)
		}
	}

	// Replace temp target directory and home directory
	f.file.Target = strings.Replace(f.file.Target, "###ZARF_TEMP###", f.componentPaths.Package, 1)
	f.file.Target = config.GetAbsHomePath(f.file.Target)

	fileList := []string{}
	if utils.IsDir(filePkgPath) {
		files, _ := utils.RecursiveFileList(filePkgPath, nil, false)
		fileList = append(fileList, files...)
	} else {
		fileList = append(fileList, filePkgPath)
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
			if err := f.valueTemplate.Apply(*f.component, subFile); err != nil {
				return fmt.Errorf("unable to template file %s: %w", subFile, err)
			}
		}
	}

	// Copy the file to the destination
	// spinner.Updatef("Saving %s", file.Target)
	err := utils.CreatePathAndCopy(filePkgPath, f.file.Target)
	if err != nil {
		return fmt.Errorf("unable to copy file %s to %s: %w", filePkgPath, f.file.Target, err)
	}

	// Loop over all symlinks and create them
	for _, link := range f.file.Symlinks {
		// spinner.Updatef("Adding symlink %s->%s", link, file.Target)
		// Try to remove the filepath if it exists
		_ = os.RemoveAll(link)
		// Make sure the parent directory exists
		_ = utils.CreateFilePath(link)
		// Create the symlink
		err := os.Symlink(f.file.Target, link)
		if err != nil {
			return fmt.Errorf("unable to create symlink %s->%s: %w", link, f.file.Target, err)
		}
	}

	// Cleanup now to reduce disk pressure
	_ = os.RemoveAll(filePkgPath)

	// spinner.Success()

	return nil
}

func (f *FileCfg) getMatrixFileMap() map[string]*FileCfg {
	matrixFiles := make(map[string]*FileCfg)
	if f.file.Matrix != nil {
		matrixValue := reflect.ValueOf(*f.file.Matrix)
		for fieldIdx := 0; fieldIdx < matrixValue.NumField(); fieldIdx++ {
			prefix := fmt.Sprintf("%s-%s", f.filePrefix, helpers.GetJSONTagName(matrixValue, fieldIdx))
			if options, ok := matrixValue.Field(fieldIdx).Interface().(*types.ZarfFileOptions); ok && options != nil {
				r := *f.file
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

				mFileCfg := New(&r, prefix, f.component, f.componentPaths).WithValues(f.valueTemplate)
				matrixFiles[prefix] = mFileCfg
			}
		}
	} else {
		matrixFiles[f.filePrefix] = f
	}

	return matrixFiles
}

func (f *FileCfg) getFilePath() (string, string) {
	rel := filepath.Join(types.FilesFolder, f.filePrefix, filepath.Base(f.file.Target))
	dst := filepath.Join(f.componentPaths.Base, rel)
	return dst, rel
}

func (f *FileCfg) finalizeFile() error {
	dst, _ := f.getFilePath()

	// Abort packaging on invalid shasum (if one is specified).
	if f.file.Shasum != "" {
		if actualShasum, _ := utils.GetCryptoHashFromFile(dst, crypto.SHA256); actualShasum != f.file.Shasum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", f.file.Source, f.file.Shasum, actualShasum)
		}
	}

	if f.file.Executable || utils.IsDir(dst) {
		_ = os.Chmod(dst, 0700)
	} else {
		_ = os.Chmod(dst, 0600)
	}
	return nil
}
