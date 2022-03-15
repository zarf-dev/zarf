package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/packager"

	"github.com/spf13/cobra"
)

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
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			if strings.Contains(args[0], "zarf-package-") || strings.Contains(args[0], "zarf-init") {
				config.DeployOptions.PackagePath = args[0]
				packager.Deploy()
				return
			}
			if args[0] == "zarf.yaml" {
				packager.Create()
				return
			}
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

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
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
