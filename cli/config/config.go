package config

import (
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
)

const K3sBinary = "/usr/local/bin/k3s"
const K3sChartPath = "/var/lib/rancher/k3s/server/static/charts"
const K3sManifestPath = "/var/lib/rancher/k3s/server/manifests"
const K3sImagePath = "/var/lib/rancher/k3s/agent/images"
const PackageInitName = "zarf-init.tar.zst"
const PackagePrefix = "zarf-package-"
const ZarfGitUser = "zarf-git-user"
const ZarfStatePath = ".zarf-state.yaml"

var CLIVersion = "unset"
var config ZarfPackage
var state ZarfState

func init() {
	if err := utils.ReadYaml(ZarfStatePath, &state); err != nil {
		state.Kind = "ZarfState"
	}
}

func IsZarfInitConfig() bool {
	return strings.ToLower(config.Kind) == "zarfinitconfig"
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

func GetState() ZarfState {
	return state
}

func GetTargetEndpoint() string {
	return state.TLS.Host
}

func WriteState(incomingState ZarfState) error {
	logrus.Debug(incomingState)
	state = incomingState
	return utils.WriteYaml(ZarfStatePath, state, 0600)
}

func LoadConfig(path string) error {
	return utils.ReadYaml(path, &config)
}

func BuildConfig(path string) error {
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
