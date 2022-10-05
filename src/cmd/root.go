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

	v.SetDefault(V_LOG_LEVEL, "info")
	v.SetDefault(V_ARCHITECTURE, "")
	v.SetDefault(V_NO_LOG_FILE, false)
	v.SetDefault(V_NO_PROGRESS, false)
	v.SetDefault(V_TMP_DIR, "")

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", v.GetString(V_LOG_LEVEL), "Log level when running Zarf. Valid options are: warn, info, debug, trace")
	rootCmd.PersistentFlags().StringVarP(&arch, "architecture", "a", v.GetString(V_ARCHITECTURE), "Architecture for OCI images")
	rootCmd.PersistentFlags().BoolVar(&message.SkipLogFile, "no-log-file", v.GetBool(V_NO_LOG_FILE), "Disable log file creation.")
	rootCmd.PersistentFlags().BoolVar(&message.NoProgress, "no-progress", v.GetBool(V_NO_PROGRESS), "Disable fancy UI progress bars, spinners, logos, etc.")
	rootCmd.PersistentFlags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", v.GetString(V_TMP_DIR), "Specify the temporary directory to use for intermediate files")
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
