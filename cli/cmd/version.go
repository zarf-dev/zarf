package cmd

import (
	"fmt"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the version the zarf binary was built from",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.CLIVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
