// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// FilePacker is a config object for packing and processing Zarf files.
type FilePacker struct {
	component      *types.ZarfComponent
	componentPaths types.ComponentPaths
	valueTemplate  *template.Values
}

// PackFiles packs all component files into a package
func (fp *FilePacker) PackFiles() error {
	for fileIdx, file := range fp.component.Files {
		message.Debugf("Loading file from %q", file.Source)

		matrixFiles := fp.getMatrixFileMap(file, fileIdx)

		for prefix, mFile := range matrixFiles {
			_, dst := fp.getFilePath(file, prefix)

			if helpers.IsURL(mFile.Source) {
				if err := utils.DownloadToFile(mFile.Source, dst, fp.component.DeprecatedCosignKeyPath); err != nil {
					return fmt.Errorf(lang.ErrDownloading, mFile.Source, err.Error())
				}
			} else {
				if err := utils.CreatePathAndCopy(mFile.Source, dst); err != nil {
					return fmt.Errorf("unable to copy file %s: %w", mFile.Source, err)
				}
			}

			if err := finalizeFile(file, dst); err != nil {
				return err
			}
		}
	}

	return nil
}

// PackSkeletonFiles packs all applicable component files into a skeleton package
func (fp *FilePacker) PackSkeletonFiles() error {
	for fileIdx, file := range fp.component.Files {
		message.Debugf("Loading file from %q", file.Source)

		matrixFiles := fp.getMatrixFileMap(file, fileIdx)

		for prefix, mFile := range matrixFiles {
			rel, dst := fp.getFilePath(file, prefix)

			if !helpers.IsURL(mFile.Source) {
				if err := utils.CreatePathAndCopy(mFile.Source, dst); err != nil {
					return fmt.Errorf("unable to copy file %s: %w", mFile.Source, err)
				}
				fp.component.Files[fileIdx].Source = rel
				if err := finalizeFile(file, dst); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// GetSBOMPaths returns paths to all component files for SBOMing
func (fp *FilePacker) GetSBOMPaths() []string {
	paths := []string{}

	for fileIdx, file := range fp.component.Files {

		matrixFiles := fp.getMatrixFileMap(file, fileIdx)

		for prefix, mFile := range matrixFiles {
			path := filepath.Join(fp.componentPaths.Files, prefix, filepath.Base(mFile.Target))
			paths = append(paths, path)
		}
	}

	return paths
}

// ProcessFiles moves files onto the host of the machine performing the deployment.
func (fp *FilePacker) ProcessFiles() error {
	// If there are no files to process, return early.
	if len(fp.component.Files) < 1 {
		return nil
	}

	spinner := message.NewProgressSpinner("Copying %d files", len(fp.component.Files))
	defer spinner.Stop()

	pkgLocation := fp.componentPaths.Files

	for fileIdx, file := range fp.component.Files {
		spinner.Updatef("Loading %s", file.Target)

		var fileLocation string
		if file.Matrix != nil {
			tag := fmt.Sprintf("%d-%s-%s", fileIdx, runtime.GOOS, runtime.GOARCH)

			matrixFiles := fp.getMatrixFileMap(file, fileIdx)
			if mFile, ok := matrixFiles[tag]; ok {
				fileLocation = filepath.Join(pkgLocation, tag, filepath.Base(mFile.Target))
			} else {
				return fmt.Errorf("the %q operating system on the %q platform is not supported by this package", runtime.GOOS, runtime.GOARCH)
			}
		} else {
			fileLocation = filepath.Join(pkgLocation, strconv.Itoa(fileIdx), filepath.Base(file.Target))
			if utils.InvalidPath(fileLocation) {
				fileLocation = filepath.Join(pkgLocation, strconv.Itoa(fileIdx))
			}
		}

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			if shasum, _ := utils.GetCryptoHashFromFile(fileLocation, crypto.SHA256); shasum != file.Shasum {
				return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", file.Source, file.Shasum, shasum)
			}
		}

		// Replace temp target directory and home directory
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", fp.componentPaths.Package, 1)
		file.Target = config.GetAbsHomePath(file.Target)

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
				spinner.Updatef("Templating %s", file.Target)
				if err := fp.valueTemplate.Apply(*fp.component, subFile, true); err != nil {
					return fmt.Errorf("unable to template file %s: %w", subFile, err)
				}
			}
		}

		// Copy the file to the destination
		spinner.Updatef("Saving %s", file.Target)
		err := utils.CreatePathAndCopy(fileLocation, file.Target)
		if err != nil {
			return fmt.Errorf("unable to copy file %s to %s: %w", fileLocation, file.Target, err)
		}

		// Loop over all symlinks and create them
		for _, link := range file.Symlinks {
			spinner.Updatef("Adding symlink %s->%s", link, file.Target)
			// Try to remove the filepath if it exists
			_ = os.RemoveAll(link)
			// Make sure the parent directory exists
			_ = utils.CreateFilePath(link)
			// Create the symlink
			err := os.Symlink(file.Target, link)
			if err != nil {
				return fmt.Errorf("unable to create symlink %s->%s: %w", link, file.Target, err)
			}
		}

		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(fileLocation)
	}

	spinner.Success()

	return nil
}

func (fp *FilePacker) getMatrixFileMap(file types.ZarfFile, fileIdx int) map[string]types.ZarfFile {
	matrixFiles := make(map[string]types.ZarfFile)
	if file.Matrix != nil {
		matrixValue := reflect.ValueOf(*file.Matrix)
		for fieldIdx := 0; fieldIdx < matrixValue.NumField(); fieldIdx++ {
			prefix := fmt.Sprintf("%d-%s", fileIdx, helpers.GetJSONTagName(matrixValue, fieldIdx))
			if options, ok := matrixValue.Field(fieldIdx).Interface().(*types.ZarfFileOptions); ok && options != nil {
				r := file
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

				matrixFiles[prefix] = r
			}
		}
	} else {
		matrixFiles[strconv.Itoa(fileIdx)] = file
	}

	return matrixFiles
}

func (fp *FilePacker) getFilePath(file types.ZarfFile, prefixKey string) (string, string) {
	rel := filepath.Join(types.FilesFolder, prefixKey, filepath.Base(file.Target))
	dst := filepath.Join(fp.componentPaths.Base, rel)
	return rel, dst
}

func finalizeFile(file types.ZarfFile, filePath string) error {
	// Abort packaging on invalid shasum (if one is specified).
	if file.Shasum != "" {
		if actualShasum, _ := utils.GetCryptoHashFromFile(filePath, crypto.SHA256); actualShasum != file.Shasum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", file.Source, file.Shasum, actualShasum)
		}
	}

	if file.Executable || utils.IsDir(filePath) {
		_ = os.Chmod(filePath, 0700)
	} else {
		_ = os.Chmod(filePath, 0600)
	}
	return nil
}
