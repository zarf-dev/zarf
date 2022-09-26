package cmd

import (
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/pterm/pterm"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logLevel string
var arch string

// Viper instance used by the cmd package
var v *viper.Viper

var rootCmd = &cobra.Command{
	Use: "zarf [COMMAND]",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setLogLevel()
		config.CliArch = arch

		// Disable progress bars for CI envs
		if os.Getenv("CI") == "true" {
			message.Debug("CI environment detected, disabling progress bars")
			message.NoProgress = true
		}
	},
	Short: "DevSecOps Airgap Toolkit",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			pterm.Println()
			if strings.Contains(args[0], "zarf-package-") || strings.Contains(args[0], "zarf-init") {
				pterm.FgYellow.Printfln("Please use \"zarf package deploy %s\" to deploy this package.", args[0])
			}
			if args[0] == "zarf.yaml" {
				pterm.FgYellow.Printfln("Please use \"zarf package create\" to create this package.")
			}
		} else {
			cmd.Help()
		}
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	initViper()

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", v.GetString("log_level"), "Log level when running Zarf. Valid options are: warn, info, debug, trace")
	rootCmd.PersistentFlags().StringVarP(&arch, "architecture", "a", v.GetString("architecture"), "Architecture for OCI images")
	rootCmd.PersistentFlags().BoolVar(&message.SkipLogFile, "no-log-file", v.GetBool("no_log_file"), "Disable log file creation.")
	rootCmd.PersistentFlags().BoolVar(&message.NoProgress, "no-progress", v.GetBool("no_progress"), "Disable fancy UI progress bars, spinners, logos, etc.")
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", v.GetString("tmpdir"), "Specify the temporary directory to use for intermediate files")

}

func initViper() {
	// Already initializedby some other command
	if v != nil {
		return
	}

	v = viper.New()
	// Specify an alternate config file
	cfgFile := os.Getenv("ZARF_CONFIG")

	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(cfgFile)
	} else {
		// Search config paths in the current directory and $HOME/.zarf.
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.zarf")
		v.SetConfigName("zarf-config")
	}

	v.SetEnvPrefix("zarf")
	v.AutomaticEnv()

	// E.g. ZARF_LOG_LEVEL=debug
	v.SetEnvPrefix("zarf")
	v.AutomaticEnv()

	// Optional, so ignore errors
	err := v.ReadInConfig()

	if err != nil {
		// Config file not found; ignore
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			message.Error(err, "Failed to read config file")
		}
	} else {
		message.Notef("Using config file %s", v.ConfigFileUsed())
	}
}

func setLogLevel() {
	match := map[string]message.LogLevel{
		"warn":  message.WarnLevel,
		"info":  message.InfoLevel,
		"debug": message.DebugLevel,
		"trace": message.TraceLevel,
	}

	// No log level set, so use the default
	if logLevel == "" {
		return
	}

	if lvl, ok := match[logLevel]; ok {
		message.SetLogLevel(lvl)
		message.Debug("Log level set to " + logLevel)
	} else {
		message.Warn("invalid log level setting")
	}
}
