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
	ZarfSeedPort           = "45000"
	ZarfRegistry           = IPV4Localhost + ":45001"
	ZarfLocalSeedRegistry  = IPV4Localhost + ":" + ZarfSeedPort

	ZarfSeedTypeCLIInject         = "cli-inject"
	ZarfSeedTypeRuntimeRegistry   = "runtime-registry"
	ZarfSeedTypeInClusterRegistry = "in-cluster-registry"

	ZarfConnectLabelName             = "zarf.dev/connect-name"
	ZarfConnectAnnotationDescription = "zarf.dev/connect-description"
	ZarfConnectAnnotationUrl         = "zarf.dev/connect-url"
)

var (
	// CLIVersion track the version of the CLI
	CLIVersion = "unset"

	// TLS options used for cert creation
	TLS types.TLSConfig

	// DeployOptions tracks user-defined values for the active deployment
	DeployOptions types.ZarfDeployOptions

	ActiveCranePlatform crane.Option

	CliArch string

	// Private vars
	config types.ZarfPackage
	state  types.ZarfState
)

func IsZarfInitConfig() bool {
	message.Debug("config.IsZarfInitConfig")
	return strings.ToLower(config.Kind) == "zarfinitconfig"
}

func SetAcrch() {
	var arch string
	if CliArch == "" {
		// If not cli override for arch, set to the package arch
		arch = config.Metadata.Architecture

		if arch == "" {
			// Finally, default to current system arch when all else fails
			arch = runtime.GOARCH
		}
	} else {
		arch = CliArch
	}

	message.Debugf("config.SetArch(%s)", arch)
	config.Build.Architecture = arch
	// Use the arch to define the image push/pull options for crane
	ActiveCranePlatform = crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: arch})
}

// GetSeedImages returns a list of image strings specified in the package, but only for init packages
func GetSeedImages() []string {
	message.Debugf("config.GetSeedImages()")
	// Only allow seed images for init config
	if IsZarfInitConfig() {
		return config.Seed
	} else {
		return []string{}
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
	return config.Data
}

func GetMetaData() types.ZarfMetadata {
	return config.Metadata
}

func GetComponents() []types.ZarfComponent {
	return config.Components
}

func SetComponents(components []types.ZarfComponent) {
	config.Components = components
}

func GetBuildData() types.ZarfBuildData {
	return config.Build
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

func GetSeedRegistry() string {
	if state.Registry.SeedType == ZarfSeedTypeCLIInject {
		return "docker.io"
	} else {
		return fmt.Sprintf("%s:%s", TLS.Host, ZarfSeedPort)
	}
}

func LoadConfig(path string) error {
	return utils.ReadYaml(path, &config)
}

func BuildConfig(path string) error {
	message.Debugf("config.BuildConfig(%v)", path)
	now := time.Now()
	currentUser, userErr := user.Current()
	hostname, hostErr := os.Hostname()

	// Record the time of package creation
	config.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with
	config.Build.Version = CLIVersion

	if hostErr == nil {
		// Record the hostname of the package creation terminal
		config.Build.Terminal = hostname
	}

	if userErr == nil {
		// Record the name of the user creating the package
		config.Build.User = currentUser.Username
	}

	return utils.WriteYaml(path, config, 0400)
}
