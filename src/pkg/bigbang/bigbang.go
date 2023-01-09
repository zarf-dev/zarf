package bigbang

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	"github.com/xanzy/go-gitlab"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

type bbImageYaml struct {
	ChartList map[string]bbImageYamlChartDef `json:"package-image-list"`
}

type bbImageYamlChartDef struct {
	Version string
	Images  []string
}

// would love for this to move to OCI soon so we can pull it from there
const DEFAULT_BIGBANG_REPO = "https://repo1.dso.mil/platform-one/big-bang/bigbang.git"

func CreateFluxComponent(bbComponent types.ZarfComponent, bbCount int) (fluxComponent types.ZarfComponent, err error) {
	fluxComponent.Name = fmt.Sprintf("flux-%d-%s", bbCount, bbComponent.BigBang.Version)

	fluxComponent.Required = bbComponent.Required

	fluxManifest := GetFluxManifest(bbComponent.BigBang.Version)
	fluxComponent.Manifests = []types.ZarfManifest{fluxManifest}

	err = importBigBangFluxImageList(bbComponent.BigBang.Version)
	if err != nil {
		return fluxComponent, fmt.Errorf("unable to import BigBang Flux image list: %w", err)
	}

	fluxComponent.Images = append(fluxComponent.Images, FluxImages[bbComponent.BigBang.Version]...)

	return fluxComponent, nil
}

// Mutates a component that should deploy BigBang by adding that version of BigBang
// as a ZarfChart
func MutateBigbangComponent(component types.ZarfComponent) (types.ZarfComponent, error) {

	tmpDir, err := utils.MakeTempDir(os.TempDir())
	if err != nil {
		return component, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	defer os.RemoveAll(tmpDir)

	repos := make([]string, 0)

	// use the default repo unless overridden
	if component.BigBang.Repo == "" {
		repos = append(repos, "https://repo1.dso.mil/platform-one/big-bang/bigbang.git")
		component.BigBang.Repo = repos[0]
	} else {
		repos = append(repos, fmt.Sprintf("%s@%s", component.BigBang.Repo, component.BigBang.Version))
	}

	// download bigbang so we can peek inside
	chart := types.ZarfChart{
		Name:        "bigbang",
		Namespace:   "bigbang",
		URL:         repos[0],
		Version:     component.BigBang.Version,
		ValuesFiles: component.BigBang.ValuesFrom,
		GitPath:     "./chart",
	}
	component.Charts = make([]types.ZarfChart, 1)
	component.Charts[0] = chart
	zarfHelmInstance := helm.Helm{
		Chart: chart,
		Cfg: &types.PackagerConfig{
			State: types.ZarfState{},
		},
		BasePath: tmpDir,
	}

	bb := zarfHelmInstance.DownloadChartFromGit("bigbang")

	zarfHelmInstance.ChartLoadOverride = bb

	template, err := zarfHelmInstance.TemplateChart()
	if err != nil {
		return component, fmt.Errorf("unable to template BigBang Chart: %w", err)
	}

	subPackageURLS := findURLs(template)
	repos[0] = fmt.Sprintf("%s@%s", repos[0], component.BigBang.Version)
	repos = append(repos, subPackageURLS...)

	component.Repos = repos

	// Get all the images.  This might be omitted here once we have this logic more globally
	// so that images are pulled from the chart annotations
	err = importBigbangImageList(component.BigBang.Version)
	if err != nil {
		return component, fmt.Errorf("unable to import bigbang image list: %w", err)
	}

	images, err := GetImages(repos)
	if err != nil {
		return component, fmt.Errorf("unable to get bb images: %w", err)
	}

	// deduple
	uniqueList := utils.Unique(images)

	component.Images = append(component.Images, uniqueList...)

	return component, nil
}

func findURLs(t string) []string {

	// Break the template into separate resources
	urls := make([]string, 0)
	yamls, _ := utils.SplitYAML([]byte(t))

	for _, y := range yamls {
		// see if its a GitRepository
		if y.GetKind() == "GitRepository" {
			url := y.Object["spec"].(map[string]interface{})["url"].(string)
			tag := y.Object["spec"].(map[string]interface{})["ref"].(map[string]interface{})["tag"].(string)
			urls = append(urls, fmt.Sprintf("%v@%v", url, tag))
		}
	}

	return urls
}

func GetImages(repos []string) ([]string, error) {

	images := make([]string, 0)
	for _, r := range repos {
		// Don't try to get images for the bigbang repo
		parts := strings.Split(r, "@")
		if len(parts) != 2 {
			continue
		}

		url := parts[0]
		version := parts[1]

		parts = strings.Split(url, "/")
		packageRepo := parts[len(parts)-1]
		packageRepo = strings.TrimSuffix(packageRepo, ".git")

		images = append(images, BigBangImages[packageRepo][version]...)

	}

	return images, nil
}

func importBigBangFluxImageList(version string) error {
	repo1, err := gitlab.NewClient("", gitlab.WithBaseURL("https://repo1.dso.mil/api/v4"))
	if err != nil {
		return fmt.Errorf("unable to create gitlab client: %w", err)
	}

	var rawFileOptions gitlab.GetRawFileOptions
	rawFileOptions.Ref = &version

	kustFile, _, err := repo1.RepositoryFiles.GetRawFile("2872", "base/flux/kustomization.yaml", &rawFileOptions, nil)
	if err != nil {
		return fmt.Errorf("unable to get flux kustomization file: %w", err)
	}

	var fluxKustomize kustypes.Kustomization

	goyaml.Unmarshal(kustFile, &fluxKustomize)

	if FluxImages[version] == nil {
		FluxImages[version] = make([]string, 0)
	}

	for _, image := range fluxKustomize.Images {
		FluxImages[version] = utils.Unique(append(FluxImages[version], fmt.Sprintf("%s:%s", image.NewName, image.NewTag)))
	}
	return nil
}

func importBigbangImageList(version string) error {
	repo1, err := gitlab.NewClient("", gitlab.WithBaseURL("https://repo1.dso.mil/api/v4"))
	if err != nil {
		return fmt.Errorf("unable to create gitlab client: %w", err)
	}

	release, _, err := repo1.Releases.GetRelease("2872", version, nil)
	if err != nil {
		return fmt.Errorf("unable to get release: %w", err)
	}

	var imagesUrl string

	for _, link := range release.Assets.Links {
		if link.Name == "package-images.yaml" {
			imagesUrl = link.URL
			break
		}
	}

	resp, err := http.Get(imagesUrl)
	if err != nil {
		return fmt.Errorf("unable to get package-images.yaml: %w", err)
	}
	defer resp.Body.Close()

	imageYamlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read package-images.yaml: %w", err)
	}

	var imageYaml bbImageYaml

	goyaml.Unmarshal(imageYamlBytes, &imageYaml)

	for chartName, versionAndImages := range imageYaml.ChartList {
		fixedChartName := bbChartNameFix(chartName)

		if BigBangImages[fixedChartName] == nil {
			BigBangImages[fixedChartName] = make(map[string][]string)
		}

		BigBangImages[fixedChartName][versionAndImages.Version] =
			utils.Unique(
				append(BigBangImages[fixedChartName][versionAndImages.Version], versionAndImages.Images...))
	}
	return nil
}

func bbChartNameFix(chartName string) string {
	switch chartName {
	case "istio":
		return "istio-controlplane"
	case "istiooperator":
		return "istio-operator"
	case "kyvernopolicies":
		return "kyverno-policies"
	case "gatekeeper":
		return "policy"
	case "clusterAuditor":
		return "cluster-auditor"
	case "eckoperator":
		return "eck-operator"
	case "logging":
		return "elasticsearch-kibana"
	case "metricsServer":
		return "metrics-server"
	default:
		return chartName
	}
}

func GetFluxManifest(version string) types.ZarfManifest {
	return types.ZarfManifest{
		Name:      "flux-system",
		Namespace: "flux-system",
		Kustomizations: []string{
			fmt.Sprintf("https://repo1.dso.mil/platform-one/big-bang/bigbang.git//base/flux?ref=%v", version),
		},
	}
}

var BigBangImages map[string]map[string][]string
var FluxImages map[string][]string

func init() {
	BigBangImages = make(map[string]map[string][]string)
	FluxImages = make(map[string][]string)
}
