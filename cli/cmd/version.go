package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the version the zarf binary was built from",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(CLIVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
