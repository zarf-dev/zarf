package config

import (
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type singleton struct {
	Viper viper.Viper
}

type RemoteBin struct {
	Name       string
	Url        string
	Shasum     string
	ArchiveExt string
}

const K3sChartPath = "/var/lib/rancher/k3s/server/static/charts"
const K3sManifestPath = "/var/lib/rancher/k3s/server/manifests"
const K3sImagePath = "/var/lib/rancher/k3s/agent/images"
const PackageInitName = "zarf-init.tar.zst"
const PackageUpdateName = "zarf-update.tar.zst"
const PackageApplianceName = "zarf-appliance-init.tar.zst"

var instance *singleton
var once sync.Once

func GetInstance() *singleton {
	once.Do(func() {
		instance = &singleton{Viper: *viper.New()}
		setupViper()
	})
	return instance
}

func IsZarfInitConfig() bool {
	var kind string
	GetInstance().Viper.UnmarshalKey("kind", &kind)
	return strings.ToLower(kind) == "zarfinitconfig"
}

func IsApplianceMode() bool {
	var mode string
	GetInstance().Viper.UnmarshalKey("mode", &mode)
	return strings.ToLower(mode) == "appliance"
}

func GetLocalBinaries() []RemoteBin {
	var binaries []RemoteBin
	GetInstance().Viper.UnmarshalKey("local.binaries", &binaries)
	return binaries
}

func GetLocalImages() []string {
	var images []string
	GetInstance().Viper.UnmarshalKey("local.images", &images)
	return images
}

func GetLocalManifests() string {
	var manifests string
	GetInstance().Viper.UnmarshalKey("local.manifestFolder", &manifests)
	return manifests
}

func GetRemoteImages() []string {
	var images []string
	GetInstance().Viper.UnmarshalKey("remote.images", &images)
	return images
}

func GetRemoteRepos() []string {
	var repos []string
	GetInstance().Viper.UnmarshalKey("remote.repos", &repos)
	return repos
}

func DynamicConfigLoad(path string) {
	logContext := logrus.WithField("path", path)
	logContext.Info("Loading dynamic config")
	GetInstance().Viper.SetConfigFile(path)
	if err := GetInstance().Viper.MergeInConfig(); err != nil {
		logContext.Warn("Unable to load the config file")
	}
}

func WriteConfig(path string) {
	if err := GetInstance().Viper.WriteConfigAs(path); err != nil {
		logrus.WithField("path", path).Fatal("Unable to write the config file")
	}
}

func setupViper() {
	instance.Viper.AddConfigPath(".")
	instance.Viper.SetConfigName("config")

	// If a config file is found, read it in.
	if err := instance.Viper.ReadInConfig(); err == nil {
		logrus.WithField("path", instance.Viper.ConfigFileUsed()).Info("Config file loaded")
	}
}
