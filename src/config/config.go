package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	GithubProject = "defenseunicorns/zarf"
	IPV4Localhost = "127.0.0.1"

	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength   = 40
	ZarfGitPushUser          = "zarf-git-user"
	ZarfGitReadUser          = "zarf-git-read-user"
	ZarfRegistryPushUser     = "zarf-push"
	ZarfRegistryPullUser     = "zarf-pull"
	ZarfImagePullSecretName  = "private-registry"
	ZarfGitServerSecretName  = "private-git-server"
	ZarfGeneratedPasswordLen = 24
	ZarfGeneratedSecretLen   = 48

	ZarfAgentHost = "agent-hook.zarf.svc"

	ZarfConnectLabelName             = "zarf.dev/connect-name"
	ZarfConnectAnnotationDescription = "zarf.dev/connect-description"
	ZarfConnectAnnotationUrl         = "zarf.dev/connect-url"

	ZarfManagedByLabel     = "app.kubernetes.io/managed-by"
	ZarfCleanupScriptsPath = "/opt/zarf"

	ZarfImageCacheDir = "images"
	ZarfGitCacheDir   = "repos"

	ZarfYAML    = "zarf.yaml"
	ZarfSBOMDir = "zarf-sbom"

	ZarfInClusterContainerRegistryURL      = "http://zarf-registry-http.zarf.svc.cluster.local:5000"
	ZarfInClusterContainerRegistryNodePort = 31999

	ZarfInClusterGitServiceURL = "http://zarf-gitea-http.zarf.svc.cluster.local:3000"

	ZarfSeedImage = "registry:2.8.1"
)

var (
	// CLIVersion track the version of the CLI
	CLIVersion = "unset"

	// CommonOptions tracks user-defined values that apply across commands.
	CommonOptions types.ZarfCommonOptions

	// CliArch is the computer architecture of the device executing the CLI commands
	CliArch string

	// ZarfSeedPort is the NodePort Zarf uses for the 'seed registry'
	ZarfSeedPort string

	// Private vars
	active types.ZarfPackage
	// Dirty Solution to getting the real time deployedComponents components.
	deployedComponents []types.DeployedComponent
	state              types.ZarfState

	SGetPublicKey string
	UIAssets      embed.FS

	// Timestamp of when the CLI was started
	operationStartTime  = time.Now().Unix()
	dataInjectionMarker = ".zarf-injection-%d"

	ZarfDefaultCachePath = filepath.Join("~", ".zarf-cache")
)

// Timestamp of when the CLI was started
func GetStartTime() int64 {
	return operationStartTime
}

func GetDataInjectionMarker() string {
	return fmt.Sprintf(dataInjectionMarker, operationStartTime)
}

func GetArch() string {
	// If CLI-orverriden then reflect that
	if CliArch != "" {
		return CliArch
	}

	if active.Metadata.Architecture != "" {
		return active.Metadata.Architecture
	}

	if active.Build.Architecture != "" {
		return active.Build.Architecture
	}

	return runtime.GOARCH
}

func GetCraneOptions() []crane.Option {
	var options []crane.Option

	// Handle insecure registry option
	if CreateOptions.Insecure {
		options = append(options, crane.Insecure)
	}

	// Add the image platform info
	options = append(options,
		crane.WithPlatform(&v1.Platform{
			OS:           "linux",
			Architecture: GetArch(),
		}),
	)

	return options
}

func GetCraneAuthOption(username string, secret string) crane.Option {
	return crane.WithAuth(
		authn.FromConfig(authn.AuthConfig{
			Username: username,
			Password: secret,
		}))
}

func GetDeployingComponents() []types.DeployedComponent {
	return deployedComponents
}

func SetDeployingComponents(components []types.DeployedComponent) {
	deployedComponents = components
}

func ClearDeployingComponents() {
	deployedComponents = []types.DeployedComponent{}
}

func SetComponents(components []types.ZarfComponent) {
	active.Components = components
}

func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

func InitState(tmpState types.ZarfState) {
	message.Debugf("config.InitState()")
	state = tmpState
}

func GetRegistry() string {
	// If a node port is populated, then we are using a registry internal to the cluster. Ignore the provided address and use localhost
	if state.RegistryInfo.NodePort >= 30000 {
		return fmt.Sprintf("%s:%d", IPV4Localhost, state.RegistryInfo.NodePort)
	}

	return state.RegistryInfo.Address
}

// GetGitServerInfo returns the GitServerInfo for the git server Zarf is configured to use from the state
func GetGitServerInfo() types.GitServerInfo {
	return state.GitServer
}

// GetAbsCachePath gets the absolute cache path for images and git repos.
func GetAbsCachePath() string {
	homePath, _ := os.UserHomeDir()

	if strings.HasPrefix(CommonOptions.CachePath, "~") {
		return strings.Replace(CommonOptions.CachePath, "~", homePath, 1)
	}
	return CommonOptions.CachePath
}

func isCompatibleComponent(component types.ZarfComponent, filterByOS bool) bool {
	message.Debugf("config.isCompatibleComponent(%s, %v)", component.Name, filterByOS)

	// Ignore only filters that are empty
	var validArch, validOS bool

	targetArch := GetArch()

	// Test for valid architecture
	if component.Only.Cluster.Architecture == "" || component.Only.Cluster.Architecture == targetArch {
		validArch = true
	} else {
		message.Debugf("Skipping component %s, %s is not compatible with %s", component.Name, component.Only.Cluster.Architecture, targetArch)
	}

	// Test for a valid OS
	if !filterByOS || component.Only.LocalOS == "" || component.Only.LocalOS == runtime.GOOS {
		validOS = true
	} else {
		message.Debugf("Skipping component %s, %s is not compatible with %s", component.Name, component.Only.LocalOS, runtime.GOOS)
	}

	return validArch && validOS
}
