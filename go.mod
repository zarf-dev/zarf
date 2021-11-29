module github.com/defenseunicorns/zarf

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/alecthomas/jsonschema v0.0.0-20211022214203-8b29eab41725
	github.com/argoproj/gitops-engine v0.5.0
	github.com/bombsimon/logrusr v1.1.0
	github.com/docker/cli v20.10.7+incompatible
	github.com/fatih/color v1.13.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v0.4.0
	github.com/goccy/go-yaml v1.9.3
	github.com/google/go-containerregistry v0.6.0
	github.com/gruntwork-io/terratest v0.38.2
	github.com/mattn/go-colorable v0.1.11
	github.com/mholt/archiver/v3 v3.5.0
	github.com/otiai10/copy v1.6.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	github.com/ulikunitz/xz v0.5.8 // indirect; CVE-2020-16845
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	helm.sh/helm/v3 v3.7.0
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
)

replace (
	github.com/argoproj/gitops-engine => github.com/defenseunicorns/gitops-engine v0.4.1
	// Just doing what https://github.com/argoproj/argo-cd/blob/v2.1.6/go.mod does for now
	k8s.io/api => k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.0
	k8s.io/apiserver => k8s.io/apiserver v0.21.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.0
	k8s.io/client-go => k8s.io/client-go v0.21.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.0
	k8s.io/code-generator => k8s.io/code-generator v0.21.0
	k8s.io/component-base => k8s.io/component-base v0.21.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.21.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.21.0
	k8s.io/cri-api => k8s.io/cri-api v0.21.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.0
	k8s.io/kubectl => k8s.io/kubectl v0.21.0
	k8s.io/kubelet => k8s.io/kubelet v0.21.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.0
	k8s.io/metrics => k8s.io/metrics v0.21.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.21.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.0
)
