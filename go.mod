module github.com/defenseunicorns/zarf

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/alecthomas/jsonschema v0.0.0-20211228220459-151e3c21f49d
	github.com/derailed/k9s v0.25.18
	github.com/distribution/distribution/v3 v3.0.0-20210804104954-38ab4c606ee3
	github.com/docker/cli v20.10.12+incompatible
	github.com/fatih/color v1.13.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v1.2.2
	github.com/goccy/go-yaml v1.9.5
	github.com/google/go-containerregistry v0.8.0
	github.com/gruntwork-io/terratest v0.38.2
	github.com/mattn/go-colorable v0.1.12
	github.com/mholt/archiver/v3 v3.5.1
	github.com/otiai10/copy v1.7.0
	github.com/pterm/pterm v0.12.33
	github.com/spf13/cobra v1.3.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20211215165025-cf75a172585e
	helm.sh/helm/v3 v3.7.2
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/klog/v2 v2.40.1
	sigs.k8s.io/yaml v1.3.0
)

replace (
	// https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-505627280
	k8s.io/api => k8s.io/api v0.22.5
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.5 // indirect
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.5 // indirect
	k8s.io/apiserver => k8s.io/apiserver v0.22.5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.5
	k8s.io/client-go => k8s.io/client-go v0.22.5
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.5
	k8s.io/code-generator => k8s.io/code-generator v0.22.5
	k8s.io/component-base => k8s.io/component-base v0.22.5
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.5
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.5
	k8s.io/cri-api => k8s.io/cri-api v0.22.5
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.5
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.5
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.5
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.5
	k8s.io/kubectl => k8s.io/kubectl v0.22.5
	k8s.io/kubelet => k8s.io/kubelet v0.22.5
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.5
	k8s.io/metrics => k8s.io/metrics v0.22.5
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.5
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.5
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.5
)
