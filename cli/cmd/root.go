package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/cli/internal/packager"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var zarfLogLevel = ""

var rootCmd = &cobra.Command{
	Use: "zarf COMMAND|ZARF-PACKAGE|ZARF-YAML",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setLogLevel(zarfLogLevel)
		if logrus.GetLevel() != logrus.InfoLevel {
			fmt.Printf("The log level has been changed to: %s\n", logrus.GetLevel())
		}
	},
	Short: "Small tool to bundle dependencies with K3s for airgapped deployments",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			if strings.Contains(args[0], "zarf-package-") {
				packager.Deploy(args[0], confirmDeploy, "")
				return
			}
			if args[0] == "zarf.yaml" {
				packager.Create(confirmCreate)
				return
			}
		}
		_ = cmd.Help()
	},
}

func Execute() {
	zarfLogo := getLogo()
	fmt.Fprintln(os.Stderr, zarfLogo)
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVarP(&zarfLogLevel, "log-level", "l", "info", "Log level when running Zarf. Valid options are: debug, info, warn, error, fatal")
}

func setLogLevel(logLevel string) {
	switch logLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		logrus.Fatalf("Unrecognized log level entry: %s", logLevel)
	}
}
