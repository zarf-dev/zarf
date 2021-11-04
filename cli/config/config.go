package config

import (
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"
)

const K3sBinary = "/usr/local/bin/k3s"
const K3sChartPath = "/var/lib/rancher/k3s/server/static/charts"
const K3sManifestPath = "/var/lib/rancher/k3s/server/manifests"
const K3sImagePath = "/var/lib/rancher/k3s/agent/images"
const PackageInitName = "zarf-init.tar.zst"
const PackagePrefix = "zarf-package-"
const ZarfLocalIP = "127.0.0.1"
const ZarfGitUser = "zarf-git-user"

var CLIVersion = "unset"

var config ZarfConfig

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

func GetMetaData() ZarfMetatdata {
	return config.Metadata
}

func GetComponents() []ZarfComponent {
	return config.Components
}

func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

func Load(path string) {
	logContext := logrus.WithField("path", path)
	logContext.Info("Loading dynamic config")
	file, err := ioutil.ReadFile(path)

	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to load the config file")
	}

	err = yaml.Unmarshal(file, &config)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to parse the config file")
	}
}

func WriteConfig(path string) {
	logContext := logrus.WithField("path", path)
	now := time.Now()
	currentUser, userErr := user.Current()
	hostname, hostErr := os.Hostname()

	// Record the time of package creation
	config.Package.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with
	config.Package.Version = CLIVersion

	if hostErr == nil {
		// Record the hostname of the package creation terminal
		config.Package.Terminal = hostname
	}

	if userErr == nil {
		// Record the name of the user creating the package
		config.Package.User = currentUser.Username
	}

	// Save the parsed output to the config path given
	content, err := yaml.Marshal(config)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to process the config data")
	}

	err = ioutil.WriteFile(path, content, 0400)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to write the config file")
	}
}
