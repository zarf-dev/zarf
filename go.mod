module github.com/defenseunicorns/zarf

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/containerd/containerd v1.5.7
	github.com/fatih/color v1.13.0
	github.com/go-git/go-git/v5 v5.4.2
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
	helm.sh/helm/v3 v3.7.0
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
)
