package config

import (
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type singleton struct {
	Viper viper.Viper
}

var instance *singleton
var once sync.Once

var printViperRead bool

func GetInstance() *singleton {
	once.Do(func() {
		instance = &singleton{Viper: *viper.New()}
		setupViper("")
	})

	return instance
}

func GetImages() []string {
	var images []string
	GetInstance().Viper.UnmarshalKey("images", &images)
	return images
}

func GetRepos() []string {
	var repos []string
	GetInstance().Viper.UnmarshalKey("repos", &repos)
	return repos
}

func DynamicConfigLoad(path string) {
	instance = &singleton{Viper: *viper.New()}
	setupViper(path)
}

func setupViper(path string) {
	if path != "" {
		instance.Viper.AddConfigPath(path)
	} else {
		instance.Viper.AddConfigPath("/etc/zarf/")
		instance.Viper.AddConfigPath(".")
	}

	instance.Viper.SetConfigName("config")

	// If a config file is found, read it in.
	if err := instance.Viper.ReadInConfig(); err == nil {
		if !printViperRead {
			logrus.WithField("path", instance.Viper.ConfigFileUsed()).Info("Config file loaded")
			printViperRead = true
		}
	}
}
