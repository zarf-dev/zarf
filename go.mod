module github.com/defenseunicorns/zarf

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/alecthomas/jsonschema v0.0.0-20211228220459-151e3c21f49d
	github.com/derailed/k9s v0.25.18
	github.com/distribution/distribution/v3 v3.0.0-20210804104954-38ab4c606ee3
	github.com/docker/cli v20.10.12+incompatible
	github.com/elazarl/goproxy v0.0.0-20190911111923-ecfe977594f1 // indirect
	github.com/fatih/color v1.13.0
	github.com/go-errors/errors v1.0.2-0.20180813162953-d98b870cc4e0 // indirect
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v1.2.2
	github.com/goccy/go-yaml v1.9.5
	github.com/google/go-containerregistry v0.8.1-0.20220209165246-a44adc326839
	github.com/mattn/go-colorable v0.1.12
	github.com/mholt/archiver/v3 v3.5.1
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/otiai10/copy v1.7.0
	github.com/pterm/pterm v0.12.33
	github.com/rancher/k3d/v5 v5.2.1
	github.com/spf13/cobra v1.3.0
	github.com/stretchr/testify v1.7.0
	github.com/testifysec/witness v0.1.7-0.20220307203322-6ec2d2b63a0e
	golang.org/x/crypto v0.0.0-20220213190939-1e6e3497d506
	helm.sh/helm/v3 v3.7.2
	k8s.io/api v0.23.4
	k8s.io/apimachinery v0.23.4
	k8s.io/cli-runtime v0.23.4 // indirect
	k8s.io/client-go v0.23.4
	k8s.io/klog/v2 v2.40.1
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/kustomize/api v0.10.1
	sigs.k8s.io/kustomize/kyaml v0.13.0
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/derailed/popeye => github.com/testifysec/popeye v0.9.9-0.20220307190025-d441c7496b07
