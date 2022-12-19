package bigbang

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// would love for this to move to OCI soon so we can pull it from there
const DEFAULT_BIGBANG_REPO = "https://repo1.dso.mil/platform-one/big-bang/bigbang.git"

func CreateFluxComponent(bbComponent types.ZarfComponent, bbCount int) (fluxComponent types.ZarfComponent) {
	fluxComponent.Name = fmt.Sprintf("flux-%d-%s", bbCount, bbComponent.BigBang.Version)

	fluxComponent.Required = bbComponent.Required

	fluxManifest := GetFluxManifest(bbComponent.BigBang.Version)
	fluxComponent.Manifests = []types.ZarfManifest{fluxManifest}

	
	fluxComponent.Images = append(fluxComponent.Images, FluxImages[bbComponent.BigBang.Version]...)

	return fluxComponent
}

// Mutates a component that should deploy BigBang by adding that version of BigBang 
// as a ZarfChart
func MutateBigbangComponent(component types.ZarfComponent) (types.ZarfComponent, error) {

	tmpDir, err := utils.MakeTempDir(os.TempDir())
	if err != nil {
		return component, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Found a Big Big Component: Version %v \n", component.BigBang.Version)
	repos := make([]string, 0)
	images := make([]string, 0)

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
		Url:         repos[0],
		Version:     component.BigBang.Version,
		ValuesFiles: component.BigBang.ValuesFrom,
		GitPath:     "./chart",
	}
	component.Charts = make([]types.ZarfChart, 1)
	component.Charts[0] = chart
	helmCfg := helm.Helm{
		Chart: chart,
		Cfg: &types.PackagerConfig{
			State: types.ZarfState{},
		},
		BasePath: tmpDir,
	}

	// I think I need this state thing
	bb := helmCfg.DownloadChartFromGit("bigbang")

	helmCfg.ChartLoadOverride = bb
	downloadedCharts := make([]string, 0)
	downloadedCharts = append(downloadedCharts, bb)
	fmt.Printf("BB Downloaded to %v\n", bb)

	template, err := helmCfg.TemplateChart()
	if err != nil {
		return component, fmt.Errorf("unable to template BigBang Chart: %w", err)
	}

	subPackageURLS := findURLs(template)
	repos[0] = fmt.Sprintf("%s@%s", repos[0], component.BigBang.Version)
	repos = append(repos, subPackageURLS...)

	component.Repos = repos

	// Get all the images.  This might be omitted here once we have this logic more globally
	// so that images are pulled from the chart annotations
	images, err = GetImages(repos)
	if err != nil {
		return component, fmt.Errorf("unable to get bb images: %w", err)
	}
	
	// deduple
	uniqueList := utils.Unique(images)

	component.Images = uniqueList

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
			fmt.Printf("Found a GitRepository: %v@%v\n", url, tag)
			urls = append(urls, fmt.Sprintf("%v@%v", url, tag))
		}
	}

	return urls
}

// need to pass in values so we can filter down...
func GetImages(repos []string) ([]string, error) {

	//check to see if images are already downloaded
	i := make([]string, 0)
	for _, r := range repos {
		parts := strings.Split(r, "@")
		if len(parts) != 2 { // should just be BigBang
			fmt.Printf("Skipping over repo %v\n", r)
			continue
		}
		url := parts[0]
		version := parts[1]

		parts = strings.Split(url, "/")
		p := parts[len(parts)-1]
		p = strings.TrimSuffix(p, ".git")

		fmt.Printf("Found Repo and Tag: %v /  %v\n", p, version)

		i = append(i, BigBangImages[p][version]...)

	}

	return i, nil
}

// // need to pass in values so we can filter down
// func GetRepos(bb types.ZarfBigBang, componentPath types.ComponentPaths) ([]string, error) {
// 	repos := make([]string, 0)
// 	if bb.Repo == "" {
// 		repos = append(repos, fmt.Sprintf("%s@%s", DEFAULT_BIGBANG_REPO, bb.Version))
// 	} else {
// 		repos = append(repos, fmt.Sprintf("%s@%s", bb.Repo, bb.Version))
// 	}

// 	//download the bigbang git repo so we can use it to parse for other git repos
// 	// Get the git repo
// 	path, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
// 	if err != nil {
// 		message.Fatalf(err, "Unable to create tmpdir: %s", config.CommonOptions.TempDirectory)
// 	}
// 	// If downloading to temp, grab all tags since the repo isn't being
// 	// packaged anyway, and it saves us from having to fetch the tags
// 	// later if we need them
// 	g.pull(gitURL, path, "")
// 	return path
// 	tempPath := gitCfg.DownloadRepoToTemp(h.Chart.Url)
// 	defer os.RemoveAll(tempPath)

// 	helmCfg := helm.Helm{
// 		Chart: types.ZarfChart{
// 			Name:        "bigbang",
// 			Url:         repos[0],
// 			ValuesFiles: bb.ValuesFrom,
// 		},
// 		Cfg: p.cfg,
// 	}

// 	_ = helmCfg.DownloadChartFromGit(componentPath.Charts)

// 	_, err := gitCfg.Pull(url, componentPath.Repos)
// 	if err != nil {

// 	}

// 	// get the list of repos from bigbang
// 	// do a helm template and pull off lines:

// }

// func MergeValues(files []string, values map[string]interface{}) (map[string]interface{}, error) {

// }

// hard coded for 1.47.0, but should be dynamic:

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

	// ServiceMesh
	BigBangImages["istio-controlplane"] = make(map[string][]string)
	BigBangImages["istio-controlplane"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.3",
		// "registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.3",
		// "registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.15.1-tetratefips-v1",
		// "registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.15.1-tetratefips-v1",
		// "registry1.dso.mil/ironbank/tetrate/istio/pilot:1.15.1-tetratefips-v1",
		// "registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.15.1-tetratefips-v1",
	}
	BigBangImages["istio-operator"] = make(map[string][]string)
	BigBangImages["istio-operator"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.3",
		// "registry1.dso.mil/ironbank/tetrate/istio/operator:1.15.1-tetratefips-v1",
	}

	// Tracing
	BigBangImages["jaeger"] = make(map[string][]string)
	BigBangImages["jaeger"]["2.37.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.39.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.39.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}

	// Observibility
	BigBangImages["kiali"] = make(map[string][]string)
	BigBangImages["kiali"]["1.58.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.58.0",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.58.0",
	}
	BigBangImages["kiali"]["1.59.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.59.1",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.59.1",
	}

	// security

	BigBangImages["clusterAuditor"] = make(map[string][]string)
	BigBangImages["clusterAuditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}

	BigBangImages["gatekeeper"] = make(map[string][]string)
	BigBangImages["gatekeeper"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	BigBangImages["gatekeeper"]["3.10.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.10.0",
	}

	BigBangImages["kyverno"] = make(map[string][]string)
	BigBangImages["kyverno"]["2.6.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.0",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	BigBangImages["kyverno"]["2.6.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.1",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	BigBangImages["kyverno-policies"] = make(map[string][]string)
	BigBangImages["kyverno-policies"]["1.0.1-bb.7"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	BigBangImages["metrics-server"] = make(map[string][]string)
	BigBangImages["metrics-server"]["3.8.0-bb.5"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes-1.21/kubectl:v1.21.14",
		"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:0.6.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
	}

	BigBangImages["twistlock"] = make(map[string][]string)
	BigBangImages["twistlock"]["0.11.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["twistlock"]["0.11.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}

	// logging
	BigBangImages["logging"] = make(map[string][]string)
	BigBangImages["logging"]["0.12.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.3",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.3",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["eckoperator"] = make(map[string][]string)
	BigBangImages["eckoperator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	BigBangImages["fluentbit"] = make(map[string][]string)
	BigBangImages["fluentbit"]["0.20.10-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:2.0.3",
	}
	BigBangImages["loki"] = make(map[string][]string)
	BigBangImages["loki"]["3.2.1-bb.3"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
	BigBangImages["loki"]["3.3.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
	BigBangImages["promtail"] = make(map[string][]string)
	BigBangImages["promtail"]["6.2.2-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/promtail:v2.6.1",
	}

	BigBangImages["tempo"] = make(map[string][]string)
	BigBangImages["tempo"]["0.16.1-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/tempo-query:1.5.0",
		"registry1.dso.mil/ironbank/opensource/grafana/tempo:1.5.0",
	}

	BigBangImages["monitoring"] = make(map[string][]string)
	BigBangImages["monitoring"]["41.7.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:9.2.2",
		"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.19.5",
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.60.1",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.60.1",
		"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.24.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.4.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.39.1",
	}
	FluxImages = make(map[string][]string)
	FluxImages["1.47.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.26.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.30.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.28.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.31.0",
	}
	FluxImages["1.48.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.26.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.30.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.28.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.31.0",
	}

}
