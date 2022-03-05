package config

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/cli/types"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	IPV4Localhost = "127.0.0.1"

	PackageInitName = "zarf-init.tar.zst"
	PackagePrefix   = "zarf-package-"

	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength = 40
	ZarfGitPushUser        = "zarf-git-user"
	ZarfRegistryPushUser   = "zarf-push"
	ZarfRegistryPullUser   = "zarf-pull"
	ZarfRegistry           = IPV4Localhost + ":45001"

	ZarfSeedTypeCLIInject         = "cli-inject"
	ZarfSeedTypeInClusterRegistry = "in-cluster-registry"

	ZarfConnectLabelName             = "zarf.dev/connect-name"
	ZarfConnectAnnotationDescription = "zarf.dev/connect-description"
	ZarfConnectAnnotationUrl         = "zarf.dev/connect-url"

	ZarfManagedByLabel = "app.kubernetes.io/managed-by"
)

var (
	// CLIVersion track the version of the CLI
	CLIVersion = "unset"

	// DeployOptions tracks user-defined values for the active deployment
	DeployOptions types.ZarfDeployOptions

	CliArch string

	ZarfSeedPort string

	// Private vars
	active types.ZarfPackage
	state  types.ZarfState
)

func IsZarfInitConfig() bool {
	message.Debug("config.IsZarfInitConfig")
	return strings.ToLower(active.Kind) == "zarfinitconfig"
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

func GetCraneOptions() crane.Option {
	return crane.WithPlatform(&v1.Platform{
		OS:           "linux",
		Architecture: GetArch(),
	})
}

func GetSeedRegistry() string {
	return fmt.Sprintf("%s:%s", IPV4Localhost, ZarfSeedPort)
}

// GetSeedImage returns a list of image strings specified in the package, but only for init packages
func GetSeedImage() string {
	message.Debugf("config.GetSeedImage()")
	// Only allow seed images for init config
	if IsZarfInitConfig() {
		return active.Seed
	} else {
		return ""
	}
}

func GetPackageName() string {
	metadata := GetMetaData()
	if metadata.Uncompressed {
		return PackagePrefix + metadata.Name + ".tar"
	} else {
		return PackagePrefix + metadata.Name + ".tar.zst"
	}
}

func GetDataInjections() []types.ZarfData {
	return active.Data
}

func GetMetaData() types.ZarfMetadata {
	return active.Metadata
}

func GetComponents() []types.ZarfComponent {
	return active.Components
}

func GetBuildData() types.ZarfBuildData {
	return active.Build
}

func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

func InitState(tmpState types.ZarfState) {
	message.Debugf("config.InitState(%v)", tmpState)
	state = tmpState
	initSecrets()
}

func GetState() types.ZarfState {
	return state
}

func GetRegistry() string {
	return fmt.Sprintf("%s:%s", IPV4Localhost, state.Registry.NodePort)
}

func LoadConfig(path string) error {
	return utils.ReadYaml(path, &active)
}

func BuildConfig(path string) error {
	message.Debugf("config.BuildConfig(%v)", path)
	now := time.Now()
	currentUser, userErr := user.Current()
	hostname, hostErr := os.Hostname()

	// Need to ensure the arch is updated if injected
	arch := GetArch()

	// normalize these for the package confirmation
	active.Metadata.Architecture = arch
	active.Build.Architecture = arch

	// Record the time of package creation
	active.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with
	active.Build.Version = CLIVersion

	if hostErr == nil {
		// Record the hostname of the package creation terminal
		active.Build.Terminal = hostname
	}

	if userErr == nil {
		// Record the name of the user creating the package
		active.Build.User = currentUser.Username
	}

	return utils.WriteYaml(path, active, 0400)
}
