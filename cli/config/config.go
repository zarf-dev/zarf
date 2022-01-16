package config

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

const IPV4Localhost = "127.0.0.1"

const (
	K3sBinary       = "/usr/local/bin/k3s"
	PackageInitName = "zarf-init.tar.zst"
	PackagePrefix   = "zarf-package-"

	ZarfGitPushUser       = "zarf-git-user"
	ZarfRegistryPushUser  = "zarf-push"
	ZarfRegistryPullUser  = "zarf-pull"
	ZarfSeedPort          = "45000"
	ZarfRegistry          = IPV4Localhost + ":45001"
	ZarfLocalSeedRegistry = IPV4Localhost + ":" + ZarfSeedPort

	ZarfSeedTypeCLIInject         = "cli-inject"
	ZarfSeedTypeRuntimeRegistry   = "runtime-registry"
	ZarfSeedTypeInClusterRegistry = "in-cluster-registry"
)

var (
	// CLIVersion track the version of the CLI
	CLIVersion = "unset"

	// TLS options used for cert creation
	TLS TLSConfig

	// DeployOptions tracks user-defined values for the active deployment
	DeployOptions ZarfDeployOptions

	// Private vars
	config ZarfPackage
	state  ZarfState
)

func IsZarfInitConfig() bool {
	return strings.ToLower(config.Kind) == "zarfinitconfig"
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

func GetDataInjections() []ZarfData {
	return config.Data
}

func GetMetaData() ZarfMetadata {
	return config.Metadata
}

func GetComponents() []ZarfComponent {
	return config.Components
}

func GetBuildData() ZarfBuildData {
	return config.Build
}

func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

func InitState(tmpState ZarfState) {
	message.Debugf("config.InitState(%v)", tmpState)
	state = tmpState
	initSecrets()
}

func GetState() ZarfState {
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
