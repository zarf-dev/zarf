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

	// BigBang

	// Initialize all possible map keys
	BigBangImages["istio-controlplane"] = make(map[string][]string)
	BigBangImages["istio-operator"] = make(map[string][]string)
	BigBangImages["jaeger"] = make(map[string][]string)
	BigBangImages["kiali"] = make(map[string][]string)
	BigBangImages["cluster-auditor"] = make(map[string][]string)
	BigBangImages["policy"] = make(map[string][]string)
	BigBangImages["kyverno"] = make(map[string][]string)
	BigBangImages["kyverno-policies"] = make(map[string][]string)
	BigBangImages["metrics-server"] = make(map[string][]string)
	BigBangImages["twistlock"] = make(map[string][]string)
	BigBangImages["elasticsearch-kibana"] = make(map[string][]string)
	BigBangImages["eck-operator"] = make(map[string][]string)
	BigBangImages["fluentbit"] = make(map[string][]string)
	BigBangImages["loki"] = make(map[string][]string)
	BigBangImages["promtail"] = make(map[string][]string)
	BigBangImages["tempo"] = make(map[string][]string)
	BigBangImages["monitoring"] = make(map[string][]string)
	BigBangImages["kyvernoreporter"] = make(map[string][]string)
	BigBangImages["neuvector"] = make(map[string][]string)
	BigBangImages["argocd"] = make(map[string][]string)
	BigBangImages["authservice"] = make(map[string][]string)
	BigBangImages["minioOperator"] = make(map[string][]string)
	BigBangImages["minio"] = make(map[string][]string)
	BigBangImages["gitlab"] = make(map[string][]string)
	BigBangImages["gitlabRunner"] = make(map[string][]string)
	BigBangImages["nexus"] = make(map[string][]string)
	BigBangImages["sonarqube"] = make(map[string][]string)
	BigBangImages["haproxy"] = make(map[string][]string)
	BigBangImages["anchore"] = make(map[string][]string)
	BigBangImages["mattermostoperator"] = make(map[string][]string)
	BigBangImages["mattermost"] = make(map[string][]string)
	BigBangImages["velero"] = make(map[string][]string)
	BigBangImages["keycloak"] = make(map[string][]string)
	BigBangImages["vault"] = make(map[string][]string)

	// Extras not in the release object?
	BigBangImages["metrics-server"]["3.8.0-bb.5"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes-1.21/kubectl:v1.21.14",
		"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:0.6.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
	}
	BigBangImages["metrics-server"]["3.8.0-bb.4"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes-1.21/kubectl:v1.21.14",
		"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:0.6.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
	}

	// Fill maps by version
	images44()
	images45()
	images46()
	images461()
	images47()
	images48()
	images49()
	images50()

	// Flux
	FluxImages = make(map[string][]string)
	FluxImages["1.44.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.24.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.28.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.26.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.29.0",
	}
	FluxImages["1.45.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.25.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.29.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.27.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.30.0",
	}
	FluxImages["1.46.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.25.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.29.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.27.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.30.0",
	}
	FluxImages["1.46.1"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.25.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.29.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.27.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.30.0",
	}
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
	FluxImages["1.49.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.27.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.31.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.29.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.32.1",
	}
	FluxImages["1.50.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.27.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.31.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.29.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.32.1",
	}

}

func images44() {
	BigBangImages["istio-controlplane"]["1.15.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.14.3-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.14.3-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.14.3-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.14.3-tetratefips-v0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.14.3-tetratefips-v0",
	}
	BigBangImages["jaeger"]["2.35.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.37.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.37.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.56.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.56.1",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.56.1",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	BigBangImages["elasticsearch-kibana"]["0.11.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.2",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.2",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["eck-operator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	BigBangImages["fluentbit"]["0.20.8-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:1.9.8",
	}
	BigBangImages["monitoring"]["40.0.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:9.1.3",
		"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.19.4",
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.59.1",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.59.1",
		"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.24.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.3.1",
		"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.38.0",
	}
	BigBangImages["twistlock"]["0.10.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["4.10.8-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.10",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.2",
	}
	BigBangImages["minioOperator"]["4.4.28-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.4.28",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.20.0",
	}
	BigBangImages["minio"]["4.4.28-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-08-26T19-53-15Z",
	}
	BigBangImages["gitlab"]["6.3.2-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.44.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.3.2",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-28T20-08-11Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-09-07T22-25-02Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.3.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.3.2",
	}
	BigBangImages["gitlabRunner"]["0.44.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.3.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.2.1",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
	}
	BigBangImages["nexus"]["41.1.0-bb.6"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.6",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.41.1-01",
	}
	BigBangImages["sonarqube"]["1.0.29-bb.4"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.9-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.19.4-bb.1"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.1.0",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.1.0",
	}
	BigBangImages["mattermostoperator"]["1.18.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.18.1",
	}
	BigBangImages["mattermost"]["7.3.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.3.0",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-08-26T19-53-15Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.10",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.8",
	}
	BigBangImages["velero"]["2.31.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.24.4",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.0",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.0",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.0",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.0",
	}
	BigBangImages["keycloak"]["18.2.1-bb.4"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["kyverno"]["2.5.3-bb.1"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.7.3",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.7.3",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
	}
	BigBangImages["kyverno-policies"]["1.0.1-bb.5"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.24.4",
	}
}

func images45() {
	BigBangImages["istio-controlplane"]["1.15.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.14.4-tetratefips-v0",
	}
	BigBangImages["jaeger"]["2.36.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.38.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.38.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.57.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.57.1",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.57.1",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	BigBangImages["elasticsearch-kibana"]["0.11.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.2",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.2",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["eck-operator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	BigBangImages["fluentbit"]["0.20.8-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:1.9.8",
	}
	BigBangImages["monitoring"]["40.4.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:9.1.6",
		"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.19.5",
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.59.2",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.59.2",
		"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.24.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.4.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.39.0",
	}
	BigBangImages["twistlock"]["0.11.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["4.10.8-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.10",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.2",
	}
	BigBangImages["minioOperator"]["4.5.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.1",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.20.4",
	}
	BigBangImages["minio"]["4.5.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-09-25T15-44-53Z",
	}
	BigBangImages["gitlab"]["6.4.1-bb.2"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.44.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.4.1",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-10-01T07-56-14Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-02T19-29-29Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.4.1",
	}
	BigBangImages["gitlabRunner"]["0.45.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.4.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.4.0",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
	}
	BigBangImages["nexus"]["42.0.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.6",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.29-bb.5"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.9-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.19.7-bb.2"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.1.1",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.1.1",
	}
	BigBangImages["mattermostoperator"]["1.18.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.18.1",
	}
	BigBangImages["mattermost"]["7.3.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.3.0",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-09-25T15-44-53Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.10",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.8",
	}
	BigBangImages["velero"]["2.31.8-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.1",
	}
	BigBangImages["keycloak"]["18.2.1-bb.4"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["kyverno"]["2.5.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.7.4",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.7.4",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
	}
	BigBangImages["kyverno-policies"]["1.0.1-bb.5"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.24.4",
	}
	BigBangImages["loki"]["1.8.10-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
	BigBangImages["promtail"]["6.2.2-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/promtail:v2.6.1",
	}
}

func images46() {
	BigBangImages["istio-controlplane"]["1.15.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.14.4-tetratefips-v0",
	}
	BigBangImages["jaeger"]["2.36.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.38.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.38.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.58.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.58.0",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.58.0",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	BigBangImages["elasticsearch-kibana"]["0.12.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.3",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.3",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/bitnami/elasticsearch-exporter:1.5.0-debian-11-r17",
	}
	BigBangImages["eck-operator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	BigBangImages["fluentbit"]["0.20.9-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:1.9.9",
	}
	BigBangImages["monitoring"]["41.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:9.2.0",
		"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.19.5",
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.60.1",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.60.1",
		"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.24.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.4.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.39.1",
	}
	BigBangImages["twistlock"]["0.11.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["5.5.7-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.12",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.2-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.2",
	}
	BigBangImages["minioOperator"]["4.5.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.3",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.21.1",
	}
	BigBangImages["minio"]["4.5.3-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-15T19-57-03Z",
	}
	BigBangImages["gitlab"]["6.4.1-bb.2"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.44.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.4.1",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-10-01T07-56-14Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-02T19-29-29Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.4.1",
	}
	BigBangImages["gitlabRunner"]["0.45.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.4.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.4.0",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
	}
	BigBangImages["nexus"]["42.0.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.6",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.29-bb.5"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.9-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.19.7-bb.2"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.1.1",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.1.1",
	}
	BigBangImages["mattermostoperator"]["1.18.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.18.1",
	}
	BigBangImages["mattermost"]["7.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.4.0",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-15T19-57-03Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.10",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.8",
	}
	BigBangImages["velero"]["2.31.8-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.1",
	}
	BigBangImages["keycloak"]["18.2.1-bb.5"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["kyverno"]["2.6.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.0",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
	}
	BigBangImages["kyverno-policies"]["1.0.1-bb.6"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	BigBangImages["loki"]["3.2.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
	BigBangImages["promtail"]["6.2.2-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/promtail:v2.6.1",
	}
}

func images461() {
	BigBangImages["istio-controlplane"]["1.15.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.0",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.14.4-tetratefips-v0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.0",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.14.4-tetratefips-v0",
	}
	BigBangImages["jaeger"]["2.36.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.38.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.38.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.58.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.58.0",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.58.0",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	BigBangImages["elasticsearch-kibana"]["0.12.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.3",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.3",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/bitnami/elasticsearch-exporter:1.5.0-debian-11-r17",
	}
	BigBangImages["eck-operator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	BigBangImages["fluentbit"]["0.20.9-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:1.9.9",
	}
	BigBangImages["monitoring"]["41.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:9.2.0",
		"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.19.5",
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.60.1",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.60.1",
		"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.24.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.4.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.39.1",
	}
	BigBangImages["twistlock"]["0.11.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["5.5.7-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.12",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.2-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.2",
	}
	BigBangImages["minioOperator"]["4.5.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.3",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.21.1",
	}
	BigBangImages["minio"]["4.5.3-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-15T19-57-03Z",
	}
	BigBangImages["gitlab"]["6.4.1-bb.2"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.44.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.4.1",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-10-01T07-56-14Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-02T19-29-29Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.4.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.4.1",
	}
	BigBangImages["gitlabRunner"]["0.45.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.4.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.4.0",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
	}
	BigBangImages["nexus"]["42.0.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.6",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.29-bb.5"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.9-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.19.7-bb.2"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.1.1",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.1.1",
	}
	BigBangImages["mattermostoperator"]["1.18.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.18.1",
	}
	BigBangImages["mattermost"]["7.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.4.0",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-15T19-57-03Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.10",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.8",
	}
	BigBangImages["velero"]["2.31.8-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.1",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.1",
	}
	BigBangImages["keycloak"]["18.2.1-bb.5"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["loki"]["3.2.1-bb.3"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
}

func images47() {
	BigBangImages["istio-controlplane"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.3",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.3",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.15.1-tetratefips-v1",
	}
	BigBangImages["jaeger"]["2.37.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.39.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.39.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.58.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.58.0",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.58.0",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	BigBangImages["elasticsearch-kibana"]["0.12.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.3",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.3",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/bitnami/elasticsearch-exporter:1.5.0-debian-11-r17",
	}
	BigBangImages["eck-operator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	BigBangImages["fluentbit"]["0.20.10-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:2.0.3",
	}
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
	BigBangImages["twistlock"]["0.11.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["5.5.7-bb.5"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.12",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.2-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.2",
	}
	BigBangImages["minioOperator"]["4.5.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.3",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.21.1",
	}
	BigBangImages["minio"]["4.5.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-08T05-27-07Z",
	}
	BigBangImages["gitlab"]["6.5.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.45.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.5.2",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-10-29T10-09-23Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-29T06-21-33Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.5.2",
	}
	BigBangImages["gitlabRunner"]["0.45.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.4.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.4.0",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.6",
	}
	BigBangImages["nexus"]["42.0.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.6",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.31-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.10-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.19.7-bb.3"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.1.1",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.1.1",
	}
	BigBangImages["mattermostoperator"]["1.18.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.18.1",
	}
	BigBangImages["mattermost"]["7.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.4.0",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-15T19-57-03Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.10",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.8",
	}
	BigBangImages["velero"]["2.31.8-bb.3"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.1",
	}
	BigBangImages["keycloak"]["18.2.1-bb.5"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["kyverno"]["2.6.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.0",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	BigBangImages["loki"]["3.2.1-bb.3"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
}

func images48() {
	BigBangImages["istio-controlplane"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.3",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.3",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.15.1-tetratefips-v1",
	}
	BigBangImages["jaeger"]["2.37.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.39.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.39.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.59.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.59.1",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.59.1",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.10.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.10.0",
	}
	BigBangImages["elasticsearch-kibana"]["0.13.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.5.0",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.5.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/bitnami/elasticsearch-exporter:1.5.0-debian-11-r17",
	}
	BigBangImages["eck-operator"]["2.5.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.5.0",
	}
	BigBangImages["fluentbit"]["0.21.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:2.0.5",
	}
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
	BigBangImages["twistlock"]["0.11.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["5.5.7-bb.5"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.12",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.3",
	}
	BigBangImages["minioOperator"]["4.5.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.4",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.21.3",
	}
	BigBangImages["minio"]["4.5.4-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-17T23-20-09Z",
	}
	BigBangImages["gitlab"]["6.5.2-bb.2"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.45.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.5.2",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-11-07T23-47-39Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-11T03-44-20Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.7",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.5.2",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.5.2",
	}
	BigBangImages["gitlabRunner"]["0.45.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.4.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.4.0",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.7",
	}
	BigBangImages["nexus"]["42.0.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.7",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.31-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.10-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.20.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.2.0",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.2.0",
	}
	BigBangImages["mattermostoperator"]["1.18.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.18.1",
	}
	BigBangImages["mattermost"]["7.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.4.0",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-10-15T19-57-03Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.10",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.8",
	}
	BigBangImages["velero"]["2.32.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.4",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.3",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.2",
	}
	BigBangImages["keycloak"]["18.2.1-bb.5"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["loki"]["3.3.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
}

func images49() {
	BigBangImages["istio-controlplane"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.3",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.3",
		"registry1.dso.mil/ironbank/tetrate/istio/operator:1.15.1-tetratefips-v1",
	}
	BigBangImages["jaeger"]["2.37.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.39.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.39.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.59.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.59.1",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.59.1",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.10.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.10.0",
	}
	BigBangImages["kyverno"]["2.6.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.1",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.1",
	}
	BigBangImages["kyverno-policies"]["1.0.1-bb.8"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.4",
	}
	BigBangImages["kyvernoreporter"]["2.13.4-bb.1"] = []string{
		"registry1.dso.mil/ironbank/nirmata/policy-reporter/policy-reporter:2.10.3",
	}
	BigBangImages["elasticsearch-kibana"]["0.13.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.5.0",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.5.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/bitnami/elasticsearch-exporter:1.5.0-debian-11-r17",
	}
	BigBangImages["eck-operator"]["2.5.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.5.0",
	}
	BigBangImages["fluentbit"]["0.21.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:2.0.5",
	}
	BigBangImages["promtail"]["6.7.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/promtail:v2.7.0",
	}
	BigBangImages["loki"]["3.6.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.7.0",
	}
	BigBangImages["neuvector"]["2.2.2-bb.2"] = []string{
		"registry1.dso.mil/ironbank/neuvector/neuvector/controller:5.0.2",
		"registry1.dso.mil/ironbank/neuvector/neuvector/enforcer:5.0.2",
		"registry1.dso.mil/ironbank/neuvector/neuvector/manager:5.0.2",
		"registry1.dso.mil/ironbank/neuvector/neuvector/scanner:latest",
	}
	BigBangImages["tempo"]["0.16.1-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/tempo-query:1.5.0",
		"registry1.dso.mil/ironbank/opensource/grafana/tempo:1.5.0",
	}
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
	BigBangImages["twistlock"]["0.11.4-bb.1"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["5.5.7-bb.5"] = []string{
		"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.12",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.3",
	}
	BigBangImages["minioOperator"]["4.5.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.4",
		"registry1.dso.mil/ironbank/opensource/minio/console:v0.21.3",
	}
	BigBangImages["minio"]["4.5.4-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-26T22-43-32Z",
	}
	BigBangImages["gitlab"]["6.6.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.45.0",
		"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
		"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.6.1",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-11-17T21-20-39Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-26T22-43-32Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.7",
		"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.6.1",
		"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.6.1",
	}
	BigBangImages["gitlabRunner"]["0.47.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.6.0",
		"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.6.0",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.7",
	}
	BigBangImages["nexus"]["42.0.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.7",
		"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.31-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.10-community",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.20.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
		"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.2.0",
		"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.2.0",
	}
	BigBangImages["mattermostoperator"]["1.19.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.19.0",
	}
	BigBangImages["mattermost"]["7.5.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.5.1",
		"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
		"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-17T23-20-09Z",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.18-1",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
	}
	BigBangImages["velero"]["2.32.2-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.4",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.3",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
		"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.2",
	}
	BigBangImages["keycloak"]["18.2.1-bb.5"] = []string{
		"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["vault"]["0.22.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/hashicorp/vault/vault-k8s:1.0.0",
		"registry1.dso.mil/ironbank/hashicorp/vault/vault:1.11.3",
	}
	BigBangImages["metrics-server"]["3.8.0-bb.6"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:0.6.1",
	}
}

func images50() {
	BigBangImages["istio-controlplane"]["1.16.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.16.1",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.16.1",
		"registry1.dso.mil/ironbank/opensource/istio/install-cni:1.16.1",
		"registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/pilot:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.15.1-tetratefips-v1",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["istio-operator"]["1.16.1-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/istio/operator:1.16.1",
			"registry1.dso.mil/ironbank/tetrate/istio/operator:1.15.1-tetratefips-v1",
	}
	BigBangImages["jaeger"]["2.37.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
			"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.39.0",
			"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.39.0",
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	BigBangImages["kiali"]["1.60.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.60.0",
			"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.60.0",
	}
	BigBangImages["cluster-auditor"]["1.5.0-bb.1"] = []string{
			"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}
	BigBangImages["policy"]["3.10.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
			"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.10.0",
	}
	BigBangImages["kyverno"]["2.6.1-bb.0"] = []string{
			"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.1",
			"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.1",
	}
	BigBangImages["kyverno-policies"]["1.0.1-bb.9"] = []string{
			"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.5",
	}
	BigBangImages["kyvernoreporter"]["2.13.4-bb.1"] = []string{
			"registry1.dso.mil/ironbank/nirmata/policy-reporter/policy-reporter:2.10.3",
	}
	BigBangImages["elasticsearch-kibana"]["0.14.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.5.2",
			"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.5.2",
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/opensource/bitnami/elasticsearch-exporter:1.5.0-debian-11-r17",
	}
	BigBangImages["eck-operator"]["2.5.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.5.0",
	}
	BigBangImages["fluentbit"]["0.21.4-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:2.0.6",
	}
	BigBangImages["promtail"]["6.7.2-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/grafana/promtail:v2.7.0",
	}
	BigBangImages["loki"]["3.7.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/grafana/loki:2.7.0",
	}
	BigBangImages["neuvector"]["2.2.2-bb.2"] = []string{
			"registry1.dso.mil/ironbank/neuvector/neuvector/controller:5.0.2",
			"registry1.dso.mil/ironbank/neuvector/neuvector/enforcer:5.0.2",
			"registry1.dso.mil/ironbank/neuvector/neuvector/manager:5.0.2",
			"registry1.dso.mil/ironbank/neuvector/neuvector/scanner:latest",
	}
	BigBangImages["tempo"]["0.16.1-bb.2"] = []string{
			"registry1.dso.mil/ironbank/opensource/grafana/tempo-query:1.5.0",
			"registry1.dso.mil/ironbank/opensource/grafana/tempo:1.5.0",
	}
	BigBangImages["monitoring"]["43.1.2-bb.0"] = []string{
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:9.3.2",
			"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.21.0",
			"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
			"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.7.0",
			"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.61.1",
			"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.61.1",
			"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.24.0",
			"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.5.0",
			"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.40.5",
	}
	BigBangImages["twistlock"]["0.11.4-bb.1"] = []string{
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
			"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	BigBangImages["argocd"]["5.5.7-bb.6"] = []string{
			"registry1.dso.mil/ironbank/big-bang/argocd:v2.4.12",
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
			"registry1.dso.mil/ironbank/opensource/dexidp/dex:v2.30.3",
	}
	BigBangImages["authservice"]["0.5.3-bb.2"] = []string{
			"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
			"registry1.dso.mil/ironbank/istio-ecosystem/authservice:0.5.3",
	}
	BigBangImages["minioOperator"]["4.5.4-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/minio/operator:v4.5.4",
			"registry1.dso.mil/ironbank/opensource/minio/console:v0.21.3",
	}
	BigBangImages["minio"]["4.5.4-bb.2"] = []string{
			"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-26T22-43-32Z",
	}
	BigBangImages["gitlab"]["6.6.1-bb.1"] = []string{
			"registry1.dso.mil/ironbank/bitnami/analytics/redis-exporter:v1.45.0",
			"registry1.dso.mil/ironbank/bitnami/redis:7.0.0-debian-10-r3",
			"registry1.dso.mil/ironbank/gitlab/gitlab/alpine-certificates:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/cfssl-self-sign:1.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitaly:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-container-registry:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-shell:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-sidekiq:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-toolbox:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-webservice:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-workhorse:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.6.1",
			"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-11-17T21-20-39Z",
			"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-26T22-43-32Z",
			"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
			"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.7",
			"registry1.dso.mil/ironbank/gitlab/gitlab/gitlab-exporter:15.6.1",
			"registry1.dso.mil/ironbank/gitlab/gitlab/kubectl:15.6.1",
	}
	BigBangImages["gitlabRunner"]["0.47.0-bb.1"] = []string{
			"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner:v15.6.0",
			"registry1.dso.mil/ironbank/gitlab/gitlab-runner/gitlab-runner-helper:v15.6.0",
			"registry1.dso.mil/ironbank/redhat/ubi/ubi8:8.7",
	}
	BigBangImages["nexus"]["42.0.0-bb.4"] = []string{
			"registry1.dso.mil/ironbank/redhat/ubi/ubi8-minimal:8.7",
			"registry1.dso.mil/ironbank/sonatype/nexus/nexus:3.42.0-01",
	}
	BigBangImages["sonarqube"]["1.0.31-bb.3"] = []string{
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/big-bang/sonarqube:8.9.10-community",
			"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
	}
	BigBangImages["haproxy"]["1.12.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/haproxy/haproxy22:v2.2.21",
	}
	BigBangImages["anchore"]["1.20.0-bb.2"] = []string{
			"registry1.dso.mil/ironbank/anchore/engine/engine:1.1.0",
			"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.12",
			"registry1.dso.mil/ironbank/anchore/enterprise/enterprise:4.2.0",
			"registry1.dso.mil/ironbank/anchore/enterpriseui/enterpriseui:4.2.0",
	}
	BigBangImages["mattermostoperator"]["1.19.0-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/mattermost/mattermost-operator:v1.19.0",
	}
	BigBangImages["mattermost"]["7.5.1-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/mattermost/mattermost:7.5.1",
			"registry1.dso.mil/ironbank/opensource/minio/mc:RELEASE.2022-08-23T05-45-20Z",
			"registry1.dso.mil/ironbank/opensource/minio/minio:RELEASE.2022-11-17T23-20-09Z",
			"registry1.dso.mil/ironbank/opensource/postgres/postgresql11:11.18-1",
			"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.13",
	}
	BigBangImages["velero"]["2.32.2-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.4",
			"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
			"registry1.dso.mil/ironbank/opensource/velero/velero:v1.9.3",
			"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-csi:v0.3.2",
			"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-aws:v1.5.2",
			"registry1.dso.mil/ironbank/opensource/velero/velero-plugin-for-microsoft-azure:v1.5.2",
	}
	BigBangImages["keycloak"]["18.2.1-bb.5"] = []string{
			"registry.dso.mil/platform-one/big-bang/apps/security-tools/keycloak/keycloak-ib:18.0.2-1.2.0-1",
			"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
			"registry1.dso.mil/ironbank/opensource/postgres/postgresql12:12.11",
	}
	BigBangImages["vault"]["0.22.1-bb.1"] = []string{
			"registry1.dso.mil/ironbank/hashicorp/vault/vault-k8s:1.0.0",
			"registry1.dso.mil/ironbank/hashicorp/vault/vault:1.11.3",
	}
	BigBangImages["metrics-server"]["3.8.3-bb.0"] = []string{
			"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:v0.6.2",
	}
}
