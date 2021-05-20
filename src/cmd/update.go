package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/git"
)

var repoPath string

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Push latest changes to the utility server",
	Run: func(cmd *cobra.Command, args []string) {
		git.Push("http://bigbang:change_me@localhost:8080", repoPath)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&repoPath, "path", "p", "", "The path for the repo to evaluate")
}
