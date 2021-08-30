package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/packager"
)

var rootCmd = &cobra.Command{
	Use:   "zarf COMMAND|ZARF-PACKAGE|ZARF-YAML",
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
		cmd.Help()
	},
}

func Execute() {
	zarfLogo := getLogo()
	fmt.Fprintln(os.Stderr, zarfLogo)
	config.Initialize()
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
