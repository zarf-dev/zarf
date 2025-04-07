package packager2

import (
	"context"
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

type InspectOptions struct {
	Cluster                 *cluster.Cluster
	Source                  string
	SkipSBOM                bool
	SkipSignatureValidation bool
	SBOMOutputDir           string
}

// Inspect inspects a package and prints its contents.
// It can be used to view the package contents without extracting it.
func Inspect(ctx context.Context, packagePath string, opt InspectOptions) error {
	// Determine the source type
	srcType, err := identifySource(opt.Source)
	if err != nil {
		if opt.Cluster == nil {
			return fmt.Errorf("failed to identify source of package: %w", err)
		}
		depPkg, err := opt.Cluster.GetDeployedPackage(ctx, opt.Source)
		if err != nil {
			return err
		}
		// check opts for skipSBOM
		// TODO: future support for retrieving SBOM from a deployed package when supported
		if !opt.SkipSBOM {

		}
		return nil
	}

	// this is a special case during inspect. do not pull the full package as it may be very large
	// default to pulling the sbom for simplicity
	if srcType == "oci" {
		path, err := pullOCIMetadata(ctx, opt.Source, tmpDir, opt.Shasum, architecture)
		if err != nil {
			return nil, err
		}
		layoutOpt := layout.PackageLayoutOptions{
			PublicKeyPath:           opt.PublicKeyPath,
			SkipSignatureValidation: opt.SkipSignatureValidation,
			IsPartial:               isPartial,
			Inspect:                 true,
		}
		pkgLayout, err := layout.LoadFromDir(ctx, path, layoutOpt)
		if err != nil {
			return nil, err
		}
		return pkgLayout, nil
	}

	loadOpt := LoadOptions{
		Source:                  opt.Source,
		SourceType:              srcType,
		SkipSignatureValidation: opt.SkipSignatureValidation,
		Architecture:            config.GetArch(),
		Filter:                  filters.Empty(),
		PublicKeyPath:           publicKeyPath,
	}
	p, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	//nolint: errcheck // ignore
	defer p.Cleanup()
	return p.Pkg, nil
}
