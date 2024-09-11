package packager2

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/sbom"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
)

type ZarfInspectOptions struct {
	// View SBOM contents while inspecting the package
	ViewSBOM bool
	// Location to output an SBOM into after package inspection
	SBOMOutputDir string
	// ListImages will list the images in the package
	ListImages bool
}

// Inspect list the contents of a package.
func Inspect(ctx context.Context, src sources.PackageSource, layout *layout.PackagePaths, options ZarfInspectOptions) (v1alpha1.ZarfPackage, error) {
	var err error
	pkg, err := getPackageMetadata(ctx, src, layout, options)
	if err != nil {
		return pkg, err
	}

	if getSBOM(ctx, options) {
		err = handleSBOMOptions(ctx, layout, pkg, options)
		if err != nil {
			return pkg, err
		}

		return pkg, nil
	}

	return pkg, nil
}

// InspectList lists the images in a component action
func InspectList(ctx context.Context, src sources.PackageSource, layout *layout.PackagePaths, options ZarfInspectOptions) ([]string, error) {
	var imageList []string
	pkg, err := getPackageMetadata(ctx, src, layout, options)
	if err != nil {
		return nil, err
	}
	// Only list images if we have have components
	if len(pkg.Components) > 0 {
		for _, component := range pkg.Components {
			imageList = append(imageList, component.Images...)
		}
		if len(imageList) > 0 {
			imageList = helpers.Unique(imageList)
			return imageList, nil
		}
		return nil, fmt.Errorf("failed listing images: list of images found in components: %d", len(imageList))
	}

	return imageList, err
}

func getPackageMetadata(ctx context.Context, src sources.PackageSource, layout *layout.PackagePaths, options ZarfInspectOptions) (v1alpha1.ZarfPackage, error) {
	SBOM := getSBOM(ctx, options)

	pkg, _, err := src.LoadPackageMetadata(ctx, layout, SBOM, true)
	if err != nil {
		return pkg, err
	}
	return pkg, nil
}

func handleSBOMOptions(_ context.Context, layout *layout.PackagePaths, pkg v1alpha1.ZarfPackage, options ZarfInspectOptions) error {
	if options.SBOMOutputDir != "" {
		out, err := layout.SBOMs.OutputSBOMFiles(options.SBOMOutputDir, pkg.Metadata.Name)
		if err != nil {
			return err
		}
		if options.ViewSBOM {
			err := sbom.ViewSBOMFiles(out)
			if err != nil {
				return err
			}
		}
	} else if options.ViewSBOM {
		err := sbom.ViewSBOMFiles(layout.SBOMs.Path)
		if err != nil {
			return err
		}
		return err
	}
	return nil
}

func getSBOM(_ context.Context, options ZarfInspectOptions) bool {
	if options.ViewSBOM || options.SBOMOutputDir != "" {
		return true
	}
	return false
}
