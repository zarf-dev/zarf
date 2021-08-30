package config

import (
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type ZarfFile struct {
	Source     string
	Shasum     string
	Target     string
	Executable bool
}

type ZarfChart struct {
	Name    string
	Url     string
	Version string
}

type ZarfFeature struct {
	Name        string
	Description string
	Default     bool
	Manifests   string
	Images      []string
	Files       []ZarfFile
	Charts      []ZarfChart
}

type ZarfMetatdata struct {
	Name         string
	Description  string
	Version      string
	Uncompressed bool
}

type ZarfContainerTarget struct {
	Namespace string
	Selector  string
	Container string
	Path      string
}
type ZarfData struct {
	Source string
	Target ZarfContainerTarget
}

const K3sBinary = "/usr/local/bin/k3s"
const K3sChartPath = "/var/lib/rancher/k3s/server/static/charts"
const K3sManifestPath = "/var/lib/rancher/k3s/server/manifests"
const K3sImagePath = "/var/lib/rancher/k3s/agent/images"
const PackageInitName = "zarf-init.tar.zst"
const PackagePrefix = "zarf-package-"
const ZarfLocal = "zarf.localhost"
const ZarfGitUser = "zarf-git-user"

func IsZarfInitConfig() bool {
	var kind string
	viper.UnmarshalKey("kind", &kind)
	return strings.ToLower(kind) == "zarfinitconfig"
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
	var data []ZarfData
	viper.UnmarshalKey("data", &data)
	return data
}

func GetMetaData() ZarfMetatdata {
	var metatdata ZarfMetatdata
	viper.UnmarshalKey("metadata.name", &metatdata.Name)
	viper.UnmarshalKey("metatdata.description", &metatdata.Description)
	viper.UnmarshalKey("metatdata.version", &metatdata.Version)
	viper.UnmarshalKey("metadata.uncompressed", &metatdata.Uncompressed)
	return metatdata
}

func GetLocalCharts() []ZarfChart {
	var charts []ZarfChart
	viper.UnmarshalKey("local.charts", &charts)
	return charts
}

func GetLocalFiles() []ZarfFile {
	var files []ZarfFile
	viper.UnmarshalKey("local.files", &files)
	return files
}

func GetLocalImages() []string {
	var images []string
	viper.UnmarshalKey("local.images", &images)
	return images
}

func GetLocalManifests() string {
	var manifests string
	viper.UnmarshalKey("local.manifests", &manifests)
	return manifests
}

func GetInitFeatures() []ZarfFeature {
	var features []ZarfFeature
	viper.UnmarshalKey("features", &features)
	return features
}

func GetRemoteImages() []string {
	var images []string
	viper.UnmarshalKey("remote.images", &images)
	return images
}

func GetRemoteRepos() []string {
	var repos []string
	viper.UnmarshalKey("remote.repos", &repos)
	return repos
}

func DynamicConfigLoad(path string) {
	logContext := logrus.WithField("path", path)
	logContext.Info("Loading dynamic config")
	file, err := os.Open(path)

	if err != nil {
		logContext.Fatal("Unable to load the config file")
	}

	err = viper.ReadConfig(file)
	if err != nil {
		logContext.Fatal("Unable to parse the config file")
	}
}

func WriteConfig(path string) {
	now := time.Now()
	currentUser, userErr := user.Current()
	hostname, hostErr := os.Hostname()

	// Record the time of package creation
	viper.Set("package.timestamp", now.Format(time.RFC1123Z))
	if hostErr == nil {
		// Record the hostname of the package creation terminal
		viper.Set("package.terminal", hostname)
	}
	if userErr == nil {
		// Record the name of the user creating the package
		viper.Set("package.user", currentUser.Name)
	}
	// Save the parsed output to the config path given
	if err := viper.WriteConfigAs(path); err != nil {
		logrus.WithField("path", path).Fatal("Unable to write the config file")
	}
}

func Initialize() {
	viper.AddConfigPath(".")
	viper.SetConfigName("zarf")

	logContext := logrus.WithField("path", viper.ConfigFileUsed())

	// If a config file is found, read it in.
	err := viper.ReadInConfig()

	if err == nil {
		logContext.Info("Config file loaded")
	}
}
