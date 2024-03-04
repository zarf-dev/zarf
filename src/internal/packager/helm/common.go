// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Helm is a config object for working with helm charts.
type Helm struct {
	chart      types.ZarfChart
	chartPath  string
	valuesPath string

	cfg       *types.PackagerConfig
	component types.ZarfComponent
	cluster   *cluster.Cluster
	timeout   time.Duration

	kubeVersion string

	chartOverride   *chart.Chart
	valuesOverrides map[string]any

	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

// Modifier is a function that modifies the Helm config.
type Modifier func(*Helm)

// New returns a new Helm config struct.
func New(chart types.ZarfChart, chartPath string, valuesPath string, mods ...Modifier) *Helm {
	h := &Helm{
		chart:      chart,
		chartPath:  chartPath,
		valuesPath: valuesPath,
		timeout:    config.ZarfDefaultHelmTimeout,
	}

	for _, mod := range mods {
		mod(h)
	}

	getter := MyRESTClientGetter{
		RESTConfig: h.cluster.K8s.RestConfig,
		CacheDir:   "/data/user/0/com.defenseunicorns.hellozarf/cache",
		RawConfig: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJSUNWN09yaHBqZEl3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBeU1qY3lNVEkzTXpsYUZ3MHpOREF5TWpReU1UTXlNemxhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUNyOTVOTFBkYXV5aGhsSkRTRXJGRzN4VFd6YUdSTkxSQmpCSXowTXJ2Q2xpM0pKTUJKbW43d2tSUmYKQnFlV2ttcVNjamkvaHVibG1UZHdhNnhsMTVYZUljdkJnZmlPYWZGalNRNHRBVXR6Z0FDU0FrSEovMWV4TXpoVwpWYTA0ZldPUTlXQU8rY1RhWXpGeWxDTUU5Sy9jdTE4T3JOd0JJdmhkMGREQkx3ZVhKSVpnK2wrOU5EQ0FGYVBRCldXczFqSVNsNVdpTm5qSXZJQVBYRm5sR0IyNm9YM2VkaDlmQjlvR09NTllVdFdjRzlGMStrRWFYK0xlZWZac28Kd2ZxZmxtUEY4UmJJME5QeVV3aTNoejBGUE40dVZEZ2pHWk5YUGV6ajJOSzZtTE1MYlhMcC95SGszKzI5UDZYagpZOWN1ZzJwWjRUR1pVbkl4Z2RKNTMyYWw2dXNQQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJTMTBFViswZWh0dHRQT213cllpNXJQeklkM3lUQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQVA5VXJLSTdQSQpsMnM2UXZwRWRLNFlEL2ZiYXlmSlp5SDRBTnBPMW5VeVROZWY2emswZ1NiNHB2Z3ZmQXQrL1d1bWhoNXNJUThzClNZRXlMZjR2Mmdyc1lYYVpvWUJhZ0lHY1NhMXd3OXdzTUVvOGRReXlEdHZTQnhjVmRUcm5TL3lKV0VMbFV6N2kKVHY0VnhMUEJQNmhweFNmOUtud2VMQ3FNUVNSMXFXOFl4dTBMUzJubzZxU1h5SDNiSnFNMEx5elQzRXBMMDBLRApMbXcxNTBiWitJR0JYa2JvS3VqZ1J5MzZFWmZzYzgvWWFIbDFtUDVlbWhGNThBbEJNYWRIVFlyMnY5cWk3Z3J3CnlrcXhXUzI4TGpLVE5qSjBVRmVYMUIrZHNER1BGQ1RlV3VSdVFYeGZQOHgxcUZHZFlvb2lITjd4a0EyczA4TlgKclREc1FDaDlwby9oCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://10.0.2.2:34645
  name: kind-kind
contexts:
- context:
    cluster: kind-kind
    user: kind-kind
  name: kind-kind
current-context: kind-kind
kind: Config
preferences: {}
users:
- name: kind-kind
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURLVENDQWhHZ0F3SUJBZ0lJZGV0WCtOc0lXaTh3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBeU1qY3lNVEkzTXpsYUZ3MHlOVEF5TWpZeU1UTXlOREJhTUR3eApIekFkQmdOVkJBb1RGbXQxWW1WaFpHMDZZMngxYzNSbGNpMWhaRzFwYm5NeEdUQVhCZ05WQkFNVEVHdDFZbVZ5CmJtVjBaWE10WVdSdGFXNHdnZ0VpTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDcUk0bnMKc2k4TS9XQTF4TldyODZ5bUJEa1kyTXBJd21IdWExOUdnL0lTZjBFNlQ1U0ZIYXhyNUhjMlp0cDd2cFpacG9oQwpkakZtZFFSRUFseXVVUmgyZ0ZscHhqOUVNamR1dHZESzNSMzlaY3R0clRCb2JQanpneVdVbVgybUxEaUdiQ1pxCkV3ZEMxWkJneENuN25lZ2s1Z0JjUmhiM0E2K1JtL2t0WnFrWk8zTnB1TFkwV2d5Q1F1S09BUklLOUhGUjI3dEIKQWpTdjlCTjFXNElZckg5aHBiSjRTN25ZUFc0NFZVQVdxaDdVb3dwN0ViSXhCTEo2c2h6R20xMlNqMVNRdjU4dQpJbVB6TFRCSEEzNXdoa0FjamE1SGw0d0w4ZGl1N1dFQjR3TGthVkdjNVdXQ0NwdFpTZk9WNExmUVM4eXd2dHJhCjRlaDVSRUdyT0JQNlQxRVZBZ01CQUFHalZqQlVNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUsKQmdnckJnRUZCUWNEQWpBTUJnTlZIUk1CQWY4RUFqQUFNQjhHQTFVZEl3UVlNQmFBRkxYUVJYN1I2RzIyMDg2YgpDdGlMbXMvTWgzZkpNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUI4N2dXKzlTcno5bmJuNUNiYm5wTFF5b0ViCkZLT2krNzU3SFdyb3hkdEVNTnRZcUE1SGp2cTJqLzhpL1dyVlhudGZzaTl3bmVRZGIzVm9EdWZMTW5hUml5MFAKdHcwZVhJSTZ3czllbmk2WUI3ejdCY1JZOTY1OFhHcXFXbFB0YmZnc2tQOUFIRW5idWJFUTl4T0l5SUpTZ3lvaAozUlpoSmsrbE9NdHJyR080ZUw0TzU3SE1XWmFBZXV6R0NoMzM5amdLeWRYTW5VbFluZUdLMXdvTUJRSktCWjB4CjBpZTFlYlc2KzhZUGJwUWtWNTZpdzRHdHd4dFY5RHFLQ2dPOThpbDBPWXFwZy9ncy9KenVrZGtuLzFwcUVRNHcKMUNMRDh3MWd4OER5OGdpVUorR3BMTVFReDlKVlRPUnBTdHl1Snh2bmlmbXAwcUE5ZjBDY2J1WGhVeFFqCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb2dJQkFBS0NBUUVBcWlPSjdMSXZEUDFnTmNUVnEvT3NwZ1E1R05qS1NNSmg3bXRmUm9QeUVuOUJPaytVCmhSMnNhK1IzTm1iYWU3NldXYWFJUW5ZeFpuVUVSQUpjcmxFWWRvQlphY1kvUkRJM2JyYnd5dDBkL1dYTGJhMHcKYUd6NDg0TWxsSmw5cGl3NGhtd21haE1IUXRXUVlNUXArNTNvSk9ZQVhFWVc5d092a1p2NUxXYXBHVHR6YWJpMgpORm9NZ2tMaWpnRVNDdlJ4VWR1N1FRSTByL1FUZFZ1Q0dLeC9ZYVd5ZUV1NTJEMXVPRlZBRnFvZTFLTUtleEd5Ck1RU3llckljeHB0ZGtvOVVrTCtmTGlKajh5MHdSd04rY0laQUhJMnVSNWVNQy9IWXJ1MWhBZU1DNUdsUm5PVmwKZ2dxYldVbnpsZUMzMEV2TXNMN2EydUhvZVVSQnF6Z1QrazlSRlFJREFRQUJBb0lCQUdiU21ZMWgxa3VjYVdPMQpkSWk1K0ZKUTVRemVIOG8vSjU1R1o4c2UvTkl1OUFYQWlIcTJoemloVjJhYVhGcEN3V3ltMTF5TFA2bXkrSVA5CmhYT1g4UmZVMDdTNEtnNFY1eWhUQ1UwZ3V2b2taZi8vbGV6V1J0SXNKUzhjWURKb01UVVQ4VmRUN3FSMm13M1EKSDZ0QU1FYjBkYTFPY3B5UUxxL3FPbm8wdStkUjZMQndGMmVONk1vNk45WklPdUo0Sk9UMzJPN0NGRWgxNVRmOApkcHpjL3U4U095dkJ2M3c5MCtmM3RpeTA0aC9KN2l0czJWdlpUdEpYMnp3a2lPRnhrMUhxZlBvSlY3aXUzR1VWCjF1Ulo1aFk4YmVZVnEwQTdyYjB4cGh6QWVUY2ZsNTM4cWswemRSd2ZpWlU3NU9ORjB6bGhYNExoKyt0NWo5ak4KV3FCTmsrRUNnWUVBd2lSUU9VWGJIRHpHY1AyS0tvZ2VUb0J0bWZMd2UxY1FyWDBCQ3B4TktUT0lTODV1VnBjVQpCNE96YWJ0eDZka2JLUFc3bkNvZWk5b3JjZWN5Z2t1Z3NMSGhzM1ZWN0g5VUIySFlsdmcrYnZ5a0V4TFB4WU9zCmtBY2lOZGdQdnRFWnUxS3FEVnJJNWNEZXB4bXR1eDBkSVBnUzdGdE1CUkxTQlBsMG1tTHdLejBDZ1lFQTRGbGIKSDF5Y3o0YkVkMHo0M1d4cUZWQ3Fld0Jsc08yaHVZZjBnZTlwa2RaODhXY1k2ajZ3emQ3emdsM3FHbU1vNkE5Qwp4K3lHREJQRnZBOGs0VFh4Y21ILzYyVXltcjQ3cE1acHFKSi9jNi8zdjE5ai9XSGFIbjBGcVUxeXhCbkU2R1lKCk5pZjdiZjdTblFKTFhkMWRiLzlOQ1FEcEdjZjRqejV3d0pVSmVya0NnWUFyTVpNYnV2d040ME92WGVtQW52cEgKTXZHdm44cDdWcHFpVHI2TmtzcmtFTkFSTmFOODVtNFJZZTduNWtsbzR1SFZFaDhrbG9ablRTbi9WSlg3UVVKRwpMYjF1aFB1Uis1NUJhamFiR0M0ajJWZlQyb3FaZ2p0QmJDVGpYK2ZZNTRMWEY0UTRKbXV1R21RdlAxcFUyQnhqCittMlRZcGllUkZxdnVxU0R1T0dBYlFLQmdDSWFNeG0vVUM3VGc0WEc4NFZrOTNNcUFlQVVuV0NabnAyL0p0R2gKZk9Db1EvSEdCQ21IUWdUcFFRYXVFK25TN09McGZ2TTQ1dDlyR2dHT0k4TUFHaVdTOC8zcU1oa0hsYlZrVzZjKwpMWlYrU0NDVTlYUU1yY1U0cCtXbVdpMm81UitXY3d3Q1k5dkNnbTFQYmZsa3J0RlpjM0pNNnlINkhiUllmM2NtCnNId3hBb0dBRkpteURyY3FGYkozc2NLZ1Z6Ti9OYUo3UlU5UitCb2hTK3BCaUJUdHdQZmdJQ1RVeC9iOHJFQkMKaC9wWEVFU2hrREpCSGppUk5mbUtEbi9MQThjVTd5dGwvRWtoZklJdkd5YStsZU5hRWQzaHVKRFBVTFdJcDZ4RgpXekdYaWM1WGMzVFVmcjZhNEFaTURSanJBVENIbkJCMkozMU9rSFhYaDJMQ1hQM2duMEk9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
`,
	}
	acfg := &action.Configuration{
		KubeClient:       kube.New(getter),
		RESTClientGetter: getter,
	}
	acfg.Init(getter, "zarf", "", func(format string, v ...interface{}) {})

	h.actionConfig = acfg

	return h
}

type MyRESTClientGetter struct {
	RESTConfig      *rest.Config
	DiscoveryClient discovery.CachedDiscoveryInterface
	CacheDir        string
	RawConfig       string
}

// ToRESTConfig returns restconfig
func (m MyRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return m.RESTConfig, nil
}

// ToDiscoveryClient returns discovery client
func (m MyRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	c, err := m.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return disk.NewCachedDiscoveryClientForConfig(c, m.CacheDir, path.Join(m.CacheDir, "http"), 1*time.Hour)
}

// ToRESTMapper returns a restmapper
func (m MyRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	disc, err := m.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disc)
	expander := restmapper.NewShortcutExpander(mapper, disc, func(s string) {})
	return expander, nil
}

// ToRawKubeConfigLoader return kubeconfig loader as-is
func (m MyRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	config, _ := clientcmd.NewClientConfigFromBytes([]byte(m.RawConfig))
	return config
}

var _ genericclioptions.RESTClientGetter = MyRESTClientGetter{}

// NewClusterOnly returns a new Helm config struct geared toward interacting with the cluster (not packages)
func NewClusterOnly(cfg *types.PackagerConfig, cluster *cluster.Cluster) *Helm {
	getter := MyRESTClientGetter{
		RESTConfig: cluster.K8s.RestConfig,
		CacheDir:   "/data/user/0/com.defenseunicorns.hellozarf/cache",
		RawConfig: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJSUNWN09yaHBqZEl3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBeU1qY3lNVEkzTXpsYUZ3MHpOREF5TWpReU1UTXlNemxhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUNyOTVOTFBkYXV5aGhsSkRTRXJGRzN4VFd6YUdSTkxSQmpCSXowTXJ2Q2xpM0pKTUJKbW43d2tSUmYKQnFlV2ttcVNjamkvaHVibG1UZHdhNnhsMTVYZUljdkJnZmlPYWZGalNRNHRBVXR6Z0FDU0FrSEovMWV4TXpoVwpWYTA0ZldPUTlXQU8rY1RhWXpGeWxDTUU5Sy9jdTE4T3JOd0JJdmhkMGREQkx3ZVhKSVpnK2wrOU5EQ0FGYVBRCldXczFqSVNsNVdpTm5qSXZJQVBYRm5sR0IyNm9YM2VkaDlmQjlvR09NTllVdFdjRzlGMStrRWFYK0xlZWZac28Kd2ZxZmxtUEY4UmJJME5QeVV3aTNoejBGUE40dVZEZ2pHWk5YUGV6ajJOSzZtTE1MYlhMcC95SGszKzI5UDZYagpZOWN1ZzJwWjRUR1pVbkl4Z2RKNTMyYWw2dXNQQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJTMTBFViswZWh0dHRQT213cllpNXJQeklkM3lUQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQVA5VXJLSTdQSQpsMnM2UXZwRWRLNFlEL2ZiYXlmSlp5SDRBTnBPMW5VeVROZWY2emswZ1NiNHB2Z3ZmQXQrL1d1bWhoNXNJUThzClNZRXlMZjR2Mmdyc1lYYVpvWUJhZ0lHY1NhMXd3OXdzTUVvOGRReXlEdHZTQnhjVmRUcm5TL3lKV0VMbFV6N2kKVHY0VnhMUEJQNmhweFNmOUtud2VMQ3FNUVNSMXFXOFl4dTBMUzJubzZxU1h5SDNiSnFNMEx5elQzRXBMMDBLRApMbXcxNTBiWitJR0JYa2JvS3VqZ1J5MzZFWmZzYzgvWWFIbDFtUDVlbWhGNThBbEJNYWRIVFlyMnY5cWk3Z3J3CnlrcXhXUzI4TGpLVE5qSjBVRmVYMUIrZHNER1BGQ1RlV3VSdVFYeGZQOHgxcUZHZFlvb2lITjd4a0EyczA4TlgKclREc1FDaDlwby9oCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://10.0.2.2:34645
  name: kind-kind
contexts:
- context:
    cluster: kind-kind
    user: kind-kind
  name: kind-kind
current-context: kind-kind
kind: Config
preferences: {}
users:
- name: kind-kind
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURLVENDQWhHZ0F3SUJBZ0lJZGV0WCtOc0lXaTh3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBeU1qY3lNVEkzTXpsYUZ3MHlOVEF5TWpZeU1UTXlOREJhTUR3eApIekFkQmdOVkJBb1RGbXQxWW1WaFpHMDZZMngxYzNSbGNpMWhaRzFwYm5NeEdUQVhCZ05WQkFNVEVHdDFZbVZ5CmJtVjBaWE10WVdSdGFXNHdnZ0VpTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDcUk0bnMKc2k4TS9XQTF4TldyODZ5bUJEa1kyTXBJd21IdWExOUdnL0lTZjBFNlQ1U0ZIYXhyNUhjMlp0cDd2cFpacG9oQwpkakZtZFFSRUFseXVVUmgyZ0ZscHhqOUVNamR1dHZESzNSMzlaY3R0clRCb2JQanpneVdVbVgybUxEaUdiQ1pxCkV3ZEMxWkJneENuN25lZ2s1Z0JjUmhiM0E2K1JtL2t0WnFrWk8zTnB1TFkwV2d5Q1F1S09BUklLOUhGUjI3dEIKQWpTdjlCTjFXNElZckg5aHBiSjRTN25ZUFc0NFZVQVdxaDdVb3dwN0ViSXhCTEo2c2h6R20xMlNqMVNRdjU4dQpJbVB6TFRCSEEzNXdoa0FjamE1SGw0d0w4ZGl1N1dFQjR3TGthVkdjNVdXQ0NwdFpTZk9WNExmUVM4eXd2dHJhCjRlaDVSRUdyT0JQNlQxRVZBZ01CQUFHalZqQlVNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUsKQmdnckJnRUZCUWNEQWpBTUJnTlZIUk1CQWY4RUFqQUFNQjhHQTFVZEl3UVlNQmFBRkxYUVJYN1I2RzIyMDg2YgpDdGlMbXMvTWgzZkpNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUI4N2dXKzlTcno5bmJuNUNiYm5wTFF5b0ViCkZLT2krNzU3SFdyb3hkdEVNTnRZcUE1SGp2cTJqLzhpL1dyVlhudGZzaTl3bmVRZGIzVm9EdWZMTW5hUml5MFAKdHcwZVhJSTZ3czllbmk2WUI3ejdCY1JZOTY1OFhHcXFXbFB0YmZnc2tQOUFIRW5idWJFUTl4T0l5SUpTZ3lvaAozUlpoSmsrbE9NdHJyR080ZUw0TzU3SE1XWmFBZXV6R0NoMzM5amdLeWRYTW5VbFluZUdLMXdvTUJRSktCWjB4CjBpZTFlYlc2KzhZUGJwUWtWNTZpdzRHdHd4dFY5RHFLQ2dPOThpbDBPWXFwZy9ncy9KenVrZGtuLzFwcUVRNHcKMUNMRDh3MWd4OER5OGdpVUorR3BMTVFReDlKVlRPUnBTdHl1Snh2bmlmbXAwcUE5ZjBDY2J1WGhVeFFqCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb2dJQkFBS0NBUUVBcWlPSjdMSXZEUDFnTmNUVnEvT3NwZ1E1R05qS1NNSmg3bXRmUm9QeUVuOUJPaytVCmhSMnNhK1IzTm1iYWU3NldXYWFJUW5ZeFpuVUVSQUpjcmxFWWRvQlphY1kvUkRJM2JyYnd5dDBkL1dYTGJhMHcKYUd6NDg0TWxsSmw5cGl3NGhtd21haE1IUXRXUVlNUXArNTNvSk9ZQVhFWVc5d092a1p2NUxXYXBHVHR6YWJpMgpORm9NZ2tMaWpnRVNDdlJ4VWR1N1FRSTByL1FUZFZ1Q0dLeC9ZYVd5ZUV1NTJEMXVPRlZBRnFvZTFLTUtleEd5Ck1RU3llckljeHB0ZGtvOVVrTCtmTGlKajh5MHdSd04rY0laQUhJMnVSNWVNQy9IWXJ1MWhBZU1DNUdsUm5PVmwKZ2dxYldVbnpsZUMzMEV2TXNMN2EydUhvZVVSQnF6Z1QrazlSRlFJREFRQUJBb0lCQUdiU21ZMWgxa3VjYVdPMQpkSWk1K0ZKUTVRemVIOG8vSjU1R1o4c2UvTkl1OUFYQWlIcTJoemloVjJhYVhGcEN3V3ltMTF5TFA2bXkrSVA5CmhYT1g4UmZVMDdTNEtnNFY1eWhUQ1UwZ3V2b2taZi8vbGV6V1J0SXNKUzhjWURKb01UVVQ4VmRUN3FSMm13M1EKSDZ0QU1FYjBkYTFPY3B5UUxxL3FPbm8wdStkUjZMQndGMmVONk1vNk45WklPdUo0Sk9UMzJPN0NGRWgxNVRmOApkcHpjL3U4U095dkJ2M3c5MCtmM3RpeTA0aC9KN2l0czJWdlpUdEpYMnp3a2lPRnhrMUhxZlBvSlY3aXUzR1VWCjF1Ulo1aFk4YmVZVnEwQTdyYjB4cGh6QWVUY2ZsNTM4cWswemRSd2ZpWlU3NU9ORjB6bGhYNExoKyt0NWo5ak4KV3FCTmsrRUNnWUVBd2lSUU9VWGJIRHpHY1AyS0tvZ2VUb0J0bWZMd2UxY1FyWDBCQ3B4TktUT0lTODV1VnBjVQpCNE96YWJ0eDZka2JLUFc3bkNvZWk5b3JjZWN5Z2t1Z3NMSGhzM1ZWN0g5VUIySFlsdmcrYnZ5a0V4TFB4WU9zCmtBY2lOZGdQdnRFWnUxS3FEVnJJNWNEZXB4bXR1eDBkSVBnUzdGdE1CUkxTQlBsMG1tTHdLejBDZ1lFQTRGbGIKSDF5Y3o0YkVkMHo0M1d4cUZWQ3Fld0Jsc08yaHVZZjBnZTlwa2RaODhXY1k2ajZ3emQ3emdsM3FHbU1vNkE5Qwp4K3lHREJQRnZBOGs0VFh4Y21ILzYyVXltcjQ3cE1acHFKSi9jNi8zdjE5ai9XSGFIbjBGcVUxeXhCbkU2R1lKCk5pZjdiZjdTblFKTFhkMWRiLzlOQ1FEcEdjZjRqejV3d0pVSmVya0NnWUFyTVpNYnV2d040ME92WGVtQW52cEgKTXZHdm44cDdWcHFpVHI2TmtzcmtFTkFSTmFOODVtNFJZZTduNWtsbzR1SFZFaDhrbG9ablRTbi9WSlg3UVVKRwpMYjF1aFB1Uis1NUJhamFiR0M0ajJWZlQyb3FaZ2p0QmJDVGpYK2ZZNTRMWEY0UTRKbXV1R21RdlAxcFUyQnhqCittMlRZcGllUkZxdnVxU0R1T0dBYlFLQmdDSWFNeG0vVUM3VGc0WEc4NFZrOTNNcUFlQVVuV0NabnAyL0p0R2gKZk9Db1EvSEdCQ21IUWdUcFFRYXVFK25TN09McGZ2TTQ1dDlyR2dHT0k4TUFHaVdTOC8zcU1oa0hsYlZrVzZjKwpMWlYrU0NDVTlYUU1yY1U0cCtXbVdpMm81UitXY3d3Q1k5dkNnbTFQYmZsa3J0RlpjM0pNNnlINkhiUllmM2NtCnNId3hBb0dBRkpteURyY3FGYkozc2NLZ1Z6Ti9OYUo3UlU5UitCb2hTK3BCaUJUdHdQZmdJQ1RVeC9iOHJFQkMKaC9wWEVFU2hrREpCSGppUk5mbUtEbi9MQThjVTd5dGwvRWtoZklJdkd5YStsZU5hRWQzaHVKRFBVTFdJcDZ4RgpXekdYaWM1WGMzVFVmcjZhNEFaTURSanJBVENIbkJCMkozMU9rSFhYaDJMQ1hQM2duMEk9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
`,
	}
	acfg := &action.Configuration{
		KubeClient:       kube.New(getter),
		RESTClientGetter: getter,
	}
	acfg.Init(getter, "zarf", "", func(format string, v ...interface{}) {})
	return &Helm{
		actionConfig: acfg,
		cfg:          cfg,
		cluster:      cluster,
		timeout:      config.ZarfDefaultHelmTimeout,
	}
}

// NewFromZarfManifest generates a helm chart and config from a given Zarf manifest.
func NewFromZarfManifest(manifest types.ZarfManifest, manifestPath, packageName, componentName string, mods ...Modifier) (h *Helm, err error) {
	spinner := message.NewProgressSpinner("Starting helm chart generation %s", manifest.Name)
	defer spinner.Stop()

	// Generate a new chart.
	tmpChart := new(chart.Chart)
	tmpChart.Metadata = new(chart.Metadata)

	// Generate a hashed chart name.
	rawChartName := fmt.Sprintf("raw-%s-%s-%s", packageName, componentName, manifest.Name)
	hasher := sha1.New()
	hasher.Write([]byte(rawChartName))
	tmpChart.Metadata.Name = rawChartName
	sha1ReleaseName := hex.EncodeToString(hasher.Sum(nil))

	// This is fun, increment forward in a semver-way using epoch so helm doesn't cry.
	tmpChart.Metadata.Version = fmt.Sprintf("0.1.%d", config.GetStartTime())
	tmpChart.Metadata.APIVersion = chart.APIVersionV1

	// Add the manifest files so helm does its thing.
	for _, file := range manifest.Files {
		spinner.Updatef("Processing %s", file)
		manifest := path.Join(manifestPath, file)
		data, err := os.ReadFile(manifest)
		if err != nil {
			return h, fmt.Errorf("unable to read manifest file %s: %w", manifest, err)
		}

		// Escape all chars and then wrap in {{ }}.
		txt := strconv.Quote(string(data))
		data = []byte("{{" + txt + "}}")

		tmpChart.Templates = append(tmpChart.Templates, &chart.File{Name: manifest, Data: data})
	}

	// Generate the struct to pass to InstallOrUpgradeChart().
	h = &Helm{
		chart: types.ZarfChart{
			Name: tmpChart.Metadata.Name,
			// Preserve the zarf prefix for chart names to match v0.22.x and earlier behavior.
			ReleaseName: fmt.Sprintf("zarf-%s", sha1ReleaseName),
			Version:     tmpChart.Metadata.Version,
			Namespace:   manifest.Namespace,
			NoWait:      manifest.NoWait,
		},
		// TODO: Figure out what belongs here
		chartPath:     manifestPath,
		chartOverride: tmpChart,
		timeout:       config.ZarfDefaultHelmTimeout,
	}

	for _, mod := range mods {
		mod(h)
	}

	spinner.Success()

	return h, nil
}

// WithDeployInfo adds the necessary information to deploy a given chart
func WithDeployInfo(component types.ZarfComponent, cfg *types.PackagerConfig, cluster *cluster.Cluster, valuesOverrides map[string]any, timeout time.Duration) Modifier {
	return func(h *Helm) {
		h.component = component
		h.cfg = cfg
		h.cluster = cluster
		h.valuesOverrides = valuesOverrides
		h.timeout = timeout
	}
}

// WithKubeVersion sets the Kube version for templating the chart
func WithKubeVersion(kubeVersion string) Modifier {
	return func(h *Helm) {
		h.kubeVersion = kubeVersion
	}
}

// StandardName generates a predictable full path for a helm chart for Zarf.
func StandardName(destination string, chart types.ZarfChart) string {
	return filepath.Join(destination, chart.Name+"-"+chart.Version)
}
