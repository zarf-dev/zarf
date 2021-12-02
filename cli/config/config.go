package config

import (
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

const K3sBinary = "/usr/local/bin/k3s"

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
	return config.Package
}

func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

func GetGitopsEndpoint() string {
	return state.TargetEndpoint
}

func GetApplianceEndpoint() string {
	return GetGitopsEndpoint() + ":45000"
}

func SetTargetEndpoint(endpoint string) error {
	state.TargetEndpoint = endpoint
	return utils.WriteYaml(ZarfStatePath, state)
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

	return utils.WriteYaml(path, config)
}
