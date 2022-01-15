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

const K3sBinary = "/usr/local/bin/k3s"
const PackageInitName = "zarf-init.tar.zst"
const PackagePrefix = "zarf-package-"
const ZarfGitPushUser = "zarf-git-user"
const ZarfRegistryPushUser = "zarf-push"
const ZarfRegistryPullUser = "zarf-pull"
const ZarfSeedPort = "45000"
const ZarfRegistry = IPV4Localhost + ":45001"
const ZarfSeedRegistry = IPV4Localhost + ":" + ZarfSeedPort

var CLIVersion = "unset"
var TLS TLSConfig
var DeployOptions ZarfDeployOptions

// private
var config ZarfPackage
var state ZarfState

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
	return fmt.Sprintf("%s:%s", TLS.Host, ZarfSeedPort)
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

func IsTLSLocalhost() bool {
	return TLS.Host == IPV4Localhost
}
