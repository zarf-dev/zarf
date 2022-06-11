package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"

	"github.com/spf13/cobra"
)

var packageConfirm bool
var zarfLogLevel = ""
var arch string

var rootCmd = &cobra.Command{
	Use: "zarf [COMMAND]|[ZARF-PACKAGE]|[ZARF-YAML]",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if zarfLogLevel != "" {
			setLogLevel(zarfLogLevel)
		}
		config.CliArch = arch
	},
	Short: "Small tool to bundle dependencies with K3s for air-gapped deployments",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Allow directly acting upon zarf assets
		if len(args) > 0 {
			config.DeployOptions.Confirm = packageConfirm
			// Root-level arguments are shortcuts to package create or deploy
			if strings.Contains(args[0], ".tar") {
				packageDeployCmd.Run(cmd, args)
			} else {
				packageCreateCmd.Run(cmd, args)
			}
			return
		}
		_ = cmd.Help()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	// Store the original cobra help func
	originalHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		// Don't show the zarf logo constantly
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)
		// Re-add the original help function
		originalHelp(c, s)
	})
	rootCmd.Flags().BoolVar(&packageConfirm, "confirm", false, "Confirm package create/deploy operation without user prompts")
	rootCmd.PersistentFlags().StringVarP(&zarfLogLevel, "log-level", "l", "", "Log level when running Zarf. Valid options are: warn, info, debug, trace")
	rootCmd.PersistentFlags().StringVarP(&arch, "architecture", "a", "", "Architecture for OCI images")
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
		message.Note("Log level set to " + logLevel)
	} else {
		message.Warn("invalid log level setting")
	}
}
