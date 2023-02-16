package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/goccy/go-yaml"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// FindImagesForChartRepo iterates over a Zarf.yaml and attempts to parse any images.
func FindImagesForChartRepo(repo, path string) ([]string, error) {
	// Also process git repos that have helm charts
	images := make([]string, 0)
	matches := strings.Split(repo, "@")
	if len(matches) < 2 {
		return images, fmt.Errorf("cannot convert git repo %s to helm chart without a version tag", repo)
	}

	// Trim the first char to match how the packager expects it, this is messy,need to clean up better
	repoHelmChartPath := strings.TrimPrefix(path, "/")

	// If a repo helm chart path is specified.
	component := types.ZarfComponent{}
	component.Charts = append(component.Charts, types.ZarfChart{
		Name:    repo,
		URL:     matches[0],
		Version: matches[1],
		GitPath: repoHelmChartPath,
	})

	tmpDir := filepath.Join(os.TempDir(), repo)
	os.Mkdir(tmpDir, 0700)
	defer os.RemoveAll(tmpDir)

	helmCfg := Helm{
		Chart:    component.Charts[0],
		BasePath: path,
		Cfg:      &types.PackagerConfig{},
	}

	helmCfg.Cfg.State = types.ZarfState{}

	// TODO (@runyontr) expand this to work for regular charts for more generic 
	// capability and pull it out from just being used by BigBang.
	downloadPath := helmCfg.DownloadChartFromGit(tmpDir)

	// Generate a new chart.
	chart, err := loader.LoadFile(downloadPath)
	if err != nil {
		return images, err
	}

	imageAnnotation := chart.Metadata.Annotations[IMAGE_KEY]

	var chartImages ChartImages

	err = yaml.Unmarshal([]byte(imageAnnotation), &chartImages)
	if err != nil {
		return images, err
	}

	for _, i := range chartImages {
		images = append(images, i.Image)
	}

	return images, nil
}

const IMAGE_KEY = "helm.sh/images"

// ChartImages captures the structure of the helm.sh/images annotation within the Helm chart.
type ChartImages []struct {
	// Name of the image.
	Name string `yaml:"name"`
	// Image with tag.
	Image string `yaml:"image"`
	// Condition specifies the values to determine if the image is included or not.
	Condition string `yaml:"condition"`
	// Dependency is the subchart that contains the image, if empty its the parent chart.
	Dependency string `yaml:"dependency"`
}
