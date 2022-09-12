package cmd

import (
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/pterm/pterm"

	"github.com/spf13/cobra"
)

var zarfLogLevel = ""
var arch string

var rootCmd = &cobra.Command{
	Use: "zarf [COMMAND]",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if zarfLogLevel != "" {
			setLogLevel(zarfLogLevel)
		}
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
	rootCmd.PersistentFlags().StringVarP(&zarfLogLevel, "log-level", "l", "", "Log level when running Zarf. Valid options are: warn, info, debug, trace")
	rootCmd.PersistentFlags().StringVarP(&arch, "architecture", "a", "", "Architecture for OCI images")
	rootCmd.PersistentFlags().BoolVar(&message.NoProgress, "no-progress", false, "Disable fancy UI progress bars, spinners, logos, etc.")
}

func setLogLevel(logLevel string) {
	match := map[string]message.LogLevel{
		"warn":  message.WarnLevel,
		"info":  message.InfoLevel,
		"debug": message.DebugLevel,
		"trace": message.TraceLevel,
	}

	if lvl, ok := match[logLevel]; ok {
		message.SetLogLevel(lvl)
		message.Debug("Log level set to " + logLevel)
	} else {
		message.Warn("invalid log level setting")
	}
}
