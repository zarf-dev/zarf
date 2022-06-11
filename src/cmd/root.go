package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager"

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
		// Allow directly acting upon zarf assets
		if len(args) > 0 {
			fileArg := args[0]

			// If this is a zarf package, try to deploy
			if strings.Contains(fileArg, "zarf-package-") || strings.Contains(fileArg, "zarf-init") {
				config.DeployOptions.PackagePath = fileArg
				packager.Deploy()
				return
			}

			// If this is a zarf.yaml, try to create the zarf package in the directory with the zarf.yaml
			if strings.Contains(fileArg, "zarf.yaml") {
				baseDir := filepath.Dir(fileArg)
				_ = os.Chdir(baseDir)
				packager.Create()
				return
			}

			// If this is a directory and has a zarf.yaml, try to create the zarf package in that directory
			dirTest := filepath.Join(fileArg, "zarf.yaml")
			if _, err := os.Stat(dirTest); err == nil {
				_ = os.Chdir(fileArg)
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
