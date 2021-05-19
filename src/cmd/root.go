package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cranecmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "shift-pack",
	Short: "Small tool to bundle dependencies with K3s for airgapped deployments",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .shift-pack.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	cranePlatformOptions := []crane.Option{
		crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"}),
	}
	rootCmd.AddCommand(cranecmd.NewCmdAuthLogin())
	rootCmd.AddCommand(cranecmd.NewCmdPull(&cranePlatformOptions))
	rootCmd.AddCommand(cranecmd.NewCmdPush(&cranePlatformOptions))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName(".shift-pack")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
