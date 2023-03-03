package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/goccy/go-yaml"
	"helm.sh/helm/v3/pkg/chart/loader"
)

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

// FindImagesForChartRepo iterates over a Zarf.yaml and attempts to parse any images.
func FindImagesForChartRepo(repo, path string) (images []string, err error) {
	matches := strings.Split(repo, "@")
	if len(matches) < 2 {
		return images, fmt.Errorf("cannot convert git repo %s to helm chart without a version tag", repo)
	}

	spinner := message.NewProgressSpinner("Discovering images in %s", repo)
	defer spinner.Stop()

	// Trim the first char to match how the packager expects it, this is messy,need to clean up better
	repoHelmChartPath := strings.TrimPrefix(path, "/")

	// If a repo helm chart path is specified.
	component := types.ZarfComponent{
		Charts: []types.ZarfChart{{
			Name:    repo,
			URL:     matches[0],
			Version: matches[1],
			GitPath: repoHelmChartPath,
		}},
	}

	helmCfg := Helm{
		Chart:    component.Charts[0],
		BasePath: path,
		Cfg: &types.PackagerConfig{
			State: types.ZarfState{},
		},
	}

	// TODO (@runyontr) expand this to work for regular charts for more generic
	// capability and pull it out from just being used by Big Bang.
	gitPath, err := helmCfg.downloadChartFromGitToTemp(spinner)
	if err != nil {
		return images, err
	}
	defer os.RemoveAll(gitPath)

	// Set the directory for the chart
	chartPath := filepath.Join(gitPath, helmCfg.Chart.GitPath)

	// Load a new chart.
	chart, err := loader.LoadDir(chartPath)
	if err != nil {
		return images, err
	}

	imageAnnotation := chart.Metadata.Annotations["helm.sh/images"]

	var chartImages ChartImages

	err = yaml.Unmarshal([]byte(imageAnnotation), &chartImages)
	if err != nil {
		return images, err
	}

	for _, i := range chartImages {
		images = append(images, i.Image)
	}

	spinner.Success()

	return images, nil
}
