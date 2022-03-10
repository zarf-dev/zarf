package validate

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/defenseunicorns/zarf/cli/types"
)

// Run performs config validations and runs message.Fatal() on errors
func Run() {
	components := config.GetComponents()

	for _, component := range components {
		for _, chart := range component.Charts {
			if err := validateChart(chart); err != nil {
				message.Fatalf(err, "Invalid chart definition in the %s component: %s", component.Name, err)
			}
		}
		for _, manifest := range component.Manifests {
			if err := validateManifest(manifest); err != nil {
				message.Fatalf(err, "Invalid manifest definition in the %s component: %s", component.Name, err)
			}
		}
	}

}

func validateChart(chart types.ZarfChart) error {
	intro := fmt.Sprintf("chart %s", chart.Name)

	// Don't allow empty names
	if chart.Name == "" {
		return fmt.Errorf("%s must include a name", intro)
	}

	// Helm max release name
	if len(chart.Name) > config.ZarfMaxChartNameLength {
		return fmt.Errorf("%s exceed the maximum length of %d characters",
			intro,
			config.ZarfMaxChartNameLength)
	}

	// Must have a namespace
	if chart.Namespace == "" {
		return fmt.Errorf("%s must include a namespace", intro)
	}

	// Must have a url
	if chart.Url == "" {
		return fmt.Errorf("%s must include a url", intro)
	}

	// Must have a version
	if chart.Version == "" {
		return fmt.Errorf("%s must include a chart version", intro)
	}

	return nil
}

func validateManifest(manifest types.ZarfManifest) error {
	intro := fmt.Sprintf("chart %s", manifest.Name)

	// Don't allow empty names
	if manifest.Name == "" {
		return fmt.Errorf("%s must include a name", intro)
	}

	// Helm max release name
	if len(manifest.Name) > config.ZarfMaxChartNameLength {
		return fmt.Errorf("%s exceed the maximum length of %d characters",
			intro,
			config.ZarfMaxChartNameLength)
	}

	// Require files in manifest
	if len(manifest.Files) < 1 && len(manifest.Kustomizations) < 1 {
		return fmt.Errorf("%s must have at least one file or kustomization", intro)
	}

	return nil
}

func ValidateImportPackage(composedComponent *types.ZarfComponent) error {
	intro := fmt.Sprintf("imported package %s", composedComponent.Name)
	path := composedComponent.Import.Path
	packageSuffix := "zarf.yaml"

	// ensure path exists
	if !(len(path) > 0) {
		return fmt.Errorf("%s must include a path", intro)
	}

	// remove zarf.yaml from path if path has zarf.yaml suffix
	if strings.HasSuffix(path, packageSuffix) {
		path = strings.Split(path, packageSuffix)[0]
	}

	// add a forward slash to end of path if it does not have one
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// ensure there is a zarf.yaml in provided path
	if utils.InvalidPath(path + packageSuffix) {
		return fmt.Errorf("invalid file path \"%s\" provided directory must contain a valid zarf.yaml file", composedComponent.Import.Path)
	}

	// replace component path with doctored path
	composedComponent.Import.Path = path
	return nil
}
