package archive

import (
	"context"
	"errors"
	"fmt"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	"github.com/mholt/archives"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CompressOpts is a placeholder for future optional Compress params
type CompressOpts struct{}

// Compress takes any number of source files and archives them into a tarball at dest path.
// FIXME(mkcp): Migrate Compress operation to use mholt/archives.
func Compress(_ context.Context, sources []string, dest string, _ CompressOpts) error {
	return archiver.Archive(sources, dest)
}

// CompressNew is a WIP
func CompressNew(ctx context.Context, sources []string, dest string, _ CompressOpts) (err error) {
	fdOpts := archives.FromDiskOptions{
		FollowSymlinks:  false,
		ClearAttributes: false,
	}
	// TODO(mkcp): Validate this, what the second param in fNames is supposed to be
	var fNames = make(map[string]string, len(sources))
	for _, source := range sources {
		fNames[source] = ""
	}
	// FIXME(mkcp): opts can be nil here
	files, err := archives.FilesFromDisk(ctx, &fdOpts, fNames)
	if err != nil {
		return err
	}
	// Open file at archive destination
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	// Ensure we close out archive dest even if compression fails
	defer func() {
		err2 := out.Close()
		err = errors.Join(err, err2)
	}()
	// TODO built format
	format := archives.CompressedArchive{
		Compression: archives.Gz{},
		Archival:    archives.Tar{},
	}
	// Compress files to archive directory
	return format.Archive(ctx, out, files)
}

// DecompressOpts provides optional parameters for Decompress
type DecompressOpts struct {
	// TODO doccomment
	UnarchiveAll bool
}

// TODO(mkcp): doccomment
// FIXME(mkcp): Migrate Decompress operation to use mholt/archives.
func Decompress(_ context.Context, sourceArchive, dest string, opts DecompressOpts) error {
	err := archiver.Unarchive(sourceArchive, dest)
	if err != nil {
		return fmt.Errorf("unable to perform decompression: %w", err)
	}
	if !opts.UnarchiveAll {
		return nil
	}
	err = filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tar") {
			dst := filepath.Join(strings.TrimSuffix(path, ".tar"), "..")
			// Unpack sboms.tar differently since it has a different folder structure than components
			if info.Name() == layout.SBOMTar {
				dst = strings.TrimSuffix(path, ".tar")
			}
			// FIXME(mkcp): support with internal/archive
			err := archiver.Unarchive(path, dst)
			if err != nil {
				return fmt.Errorf(lang.ErrUnarchive, path, err.Error())
			}
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf(lang.ErrRemoveFile, path, err.Error())
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to unarchive all nested tarballs: %w", err)
	}
	return nil
}

// RenameFromMetadata renames a tarball based on its metadata.
// FIXME(mkcp): Simplify, extract out packager-specific stuff
func RenameFromMetadata(path string) (string, error) {
	var pkg v1alpha1.ZarfPackage

	ext := filepath.Ext(path)
	if ext == "" {
		pathWithExt, err := identifyUnknownTarball(path)
		if err != nil {
			return "", err
		}
		path = pathWithExt
		ext = filepath.Ext(path)
	}
	if ext == ".zst" {
		ext = ".tar.zst"
	}

	// FIXME(mkcp): Migrate to mholt/archives
	if err := archiver.Walk(path, func(f archiver.File) error {
		if f.Name() == layout.ZarfYAML {
			b, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			if err := goyaml.Unmarshal(b, &pkg); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return "", err
	}

	if pkg.Metadata.Name == "" {
		return "", fmt.Errorf("%q does not contain a zarf.yaml", path)
	}

	name := NameFromMetadata(&pkg, false)

	name = fmt.Sprintf("%s%s", name, ext)

	tb := filepath.Join(filepath.Dir(path), name)

	return tb, os.Rename(path, tb)
}

func identifyUnknownTarball(path string) (string, error) {
	if helpers.InvalidPath(path) {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if filepath.Ext(path) != "" && isValidFileExtension(path) {
		return path, nil
	} else if filepath.Ext(path) != "" && !isValidFileExtension(path) {
		return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, getValidPackageExtensions())
	}

	// rename to .tar.zst and check if it's a valid tar.zst
	tzst := fmt.Sprintf("%s.tar.zst", path)
	if err := os.Rename(path, tzst); err != nil {
		return "", err
	}
	// FIXME(mkcp): Support with internal/archive
	format, err := archiver.ByExtension(tzst)
	if err != nil {
		return "", err
	}
	_, ok := format.(*archiver.TarZstd)
	if ok {
		return tzst, nil
	}

	// rename to .tar and check if it's a valid tar
	tb := fmt.Sprintf("%s.tar", path)
	if err := os.Rename(tzst, tb); err != nil {
		return "", err
	}
	// FIXME(mkcp): Migrate to mholt/archives
	format, err = archiver.ByExtension(tb)
	if err != nil {
		return "", err
	}
	_, ok = format.(*archiver.Tar)
	if ok {
		return tb, nil
	}

	return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, getValidPackageExtensions())
}

// getValidPackageExtensions returns the valid package extensions.
// NOTE(mkcp): Similar to archives format
func getValidPackageExtensions() [2]string {
	return [...]string{".tar.zst", ".tar"}
}

// IsValidFileExtension returns true if the filename has a valid package extension.
func isValidFileExtension(filename string) bool {
	for _, extension := range getValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			return true
		}
	}

	return false
}

// NameFromMetadata generates a name from a package's metadata.
// FIXME(mkcp) Lots of packager-specific details here, figure out where this lives in packager2.
func NameFromMetadata(pkg *v1alpha1.ZarfPackage, isSkeleton bool) string {
	var name string

	arch := config.GetArch(pkg.Metadata.Architecture, pkg.Build.Architecture)

	if isSkeleton {
		arch = zoci.SkeletonArch
	}

	switch pkg.Kind {
	case v1alpha1.ZarfInitConfig:
		name = fmt.Sprintf("zarf-init-%s", arch)
	case v1alpha1.ZarfPackageConfig:
		name = fmt.Sprintf("zarf-package-%s-%s", pkg.Metadata.Name, arch)
	default:
		name = fmt.Sprintf("zarf-%s-%s", strings.ToLower(string(pkg.Kind)), arch)
	}

	if pkg.Build.Differential {
		name = fmt.Sprintf("%s-%s-differential-%s", name, pkg.Build.DifferentialPackageVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		name = fmt.Sprintf("%s-%s", name, pkg.Metadata.Version)
	}

	return name
}
