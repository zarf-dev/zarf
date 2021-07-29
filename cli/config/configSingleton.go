package config

import (
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type singleton struct {
	Viper viper.Viper
}

type ZarfFile struct {
	Url        string
	Shasum     string
	Target     string
	Executable bool
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

func GetLocalFiles() []ZarfFile {
	var files []ZarfFile
	GetInstance().Viper.UnmarshalKey("local.files", &files)
	return files
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
	now := time.Now()
	user, userErr := user.Current()
	hostname, hostErr := os.Hostname()

	// Record the time of package creation
	GetInstance().Viper.Set("package.timestamp", now.Format(time.RFC1123Z))
	if hostErr == nil {
		// Record the hostname of the package creation terminal
		GetInstance().Viper.Set("package.terminal", hostname)
	}
	if userErr == nil {
		// Record the name of the user creating the package
		GetInstance().Viper.Set("package.user", user.Name)
	}
	// Save the parsed output to the config path given
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
