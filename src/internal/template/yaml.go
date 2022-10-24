package template

import (
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// ProcessYamlFilesInPath iterates over all yaml files in a given path and performs Zarf templating + image swapping
func ProcessYamlFilesInPath(path string, component types.ZarfComponent) []string {
	// Only pull in yml and yaml files
	pattern := regexp.MustCompile(`(?mi)\.ya?ml$`)
	manifests, _ := utils.RecursiveFileList(path, pattern)
	valueTemplate := Generate()

	for _, manifest := range manifests {
		valueTemplate.Apply(component, manifest)
	}

	return manifests
}
