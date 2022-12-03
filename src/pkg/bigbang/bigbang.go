package bigbang

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
)

// would love for this to move to OCI soon so we can pull it from there
const DEFAULT_BIGBANG_REPO = "https://repo1.dso.mil/platform-one/big-bang/bigbang.git"

// hard coded for now, but should be dynamic

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

		i = append(i, Images[p][version]...)

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

var Images map[string]map[string][]string

func init() {
	Images = make(map[string]map[string][]string)

	// ServiceMesh
	Images["istio-controlplane"] = make(map[string][]string)
	Images["istio-controlplane"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.15.3",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.15.3",
		// "registry1.dso.mil/ironbank/opensource/istio/install-cni:1.15.3",
		// "registry1.dso.mil/ironbank/tetrate/istio/istioctl:1.15.1-tetratefips-v1",
		// "registry1.dso.mil/ironbank/tetrate/istio/proxyv2:1.15.1-tetratefips-v1",
		// "registry1.dso.mil/ironbank/tetrate/istio/pilot:1.15.1-tetratefips-v1",
		// "registry1.dso.mil/ironbank/tetrate/istio/install-cni:1.15.1-tetratefips-v1",
	}
	Images["istio-operator"] = make(map[string][]string)
	Images["istio-operator"]["1.15.3-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.15.3",
		// "registry1.dso.mil/ironbank/tetrate/istio/operator:1.15.1-tetratefips-v1",
	}

	// Tracing
	Images["jaeger"] = make(map[string][]string)
	Images["jaeger"]["2.37.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/all-in-one:1.39.0",
		"registry1.dso.mil/ironbank/opensource/jaegertracing/jaeger-operator:1.39.0",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}

	// Observibility
	Images["kiali"] = make(map[string][]string)
	Images["kiali"]["1.58.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.58.0",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.58.0",
	}
	Images["kiali"]["1.59.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.59.1",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.59.1",
	}

	// security

	Images["clusterAuditor"] = make(map[string][]string)
	Images["clusterAuditor"]["1.5.0-bb.1"] = []string{
		"registry1.dso.mil/ironbank/bigbang/cluster-auditor/opa-exporter:v0.0.7",
	}

	Images["gatekeeper"] = make(map[string][]string)
	Images["gatekeeper"]["3.9.0-bb.3"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.2",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.9.0",
	}
	Images["gatekeeper"]["3.10.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
		"registry1.dso.mil/ironbank/opensource/openpolicyagent/gatekeeper:v3.10.0",
	}

	Images["kyverno"] = make(map[string][]string)
	Images["kyverno"]["2.6.0-bb.2"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.0",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	Images["kyverno"]["2.6.1-bb.0"] = []string{
		"registry1.dso.mil/ironbank/nirmata/kyverno:v1.8.1",
		"registry1.dso.mil/ironbank/nirmata/kyvernopre:v1.8.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	Images["kyverno-policies"] = make(map[string][]string)
	Images["kyverno-policies"]["1.0.1-bb.7"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.25.3",
	}
	Images["metrics-server"] = make(map[string][]string)
	Images["metrics-server"]["3.8.0-bb.5"] = []string{
		"registry1.dso.mil/ironbank/opensource/kubernetes-1.21/kubectl:v1.21.14",
		"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:0.6.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.6.0",
	}

	Images["twistlock"] = make(map[string][]string)
	Images["twistlock"]["0.11.3-bb.2"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}
	Images["twistlock"]["0.11.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
		"registry1.dso.mil/ironbank/twistlock/console/console:22.06.197",
		"registry1.dso.mil/ironbank/twistlock/defender/defender:22.06.197",
	}

	// logging
	Images["logging"] = make(map[string][]string)
	Images["logging"]["0.12.1-bb.1"] = []string{
		"registry1.dso.mil/ironbank/elastic/elasticsearch/elasticsearch:8.4.3",
		"registry1.dso.mil/ironbank/elastic/kibana/kibana:8.4.3",
		"registry1.dso.mil/ironbank/big-bang/base:2.0.0",
	}
	Images["eckoperator"] = make(map[string][]string)
	Images["eckoperator"]["2.4.0-bb.0"] = []string{
		"registry1.dso.mil/ironbank/elastic/eck-operator/eck-operator:2.4.0",
	}
	Images["fluentbit"] = make(map[string][]string)
	Images["fluentbit"]["0.20.10-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/fluent/fluent-bit:2.0.3",
	}
	Images["loki"] = make(map[string][]string)
	Images["loki"]["3.2.1-bb.3"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
	Images["loki"]["3.3.4-bb.0"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/loki:2.6.1",
	}
	Images["promtail"] = make(map[string][]string)
	Images["promtail"]["6.2.2-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/promtail:v2.6.1",
	}

	Images["tempo"] = make(map[string][]string)
	Images["tempo"]["0.16.1-bb.2"] = []string{
		"registry1.dso.mil/ironbank/opensource/grafana/tempo-query:1.5.0",
		"registry1.dso.mil/ironbank/opensource/grafana/tempo:1.5.0",
	}

	Images["monitoring"] = make(map[string][]string)
	Images["monitoring"]["41.7.3-bb.0"] = []string{
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
	Images["flux"] = make(map[string][]string)
	Images["flux"]["1.47.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.26.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.30.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.28.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.31.0",
	}
	Images["flux"]["1.48.0"] = []string{
		"registry1.dso.mil/ironbank/fluxcd/helm-controller:v0.26.0",
		"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v0.30.0",
		"registry1.dso.mil/ironbank/fluxcd/notification-controller:v0.28.0",
		"registry1.dso.mil/ironbank/fluxcd/source-controller:v0.31.0",
	}

}
