package cmd

import (
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
)

var gitUrlPath string
var repoPath string

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Push latest changes to the utility server",
	Run: func(cmd *cobra.Command, args []string) {
		git.PushAllDirectories(gitUrlPath, repoPath)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&gitUrlPath, "url", "u", "", "The git server url with auth if needed, e.g. \"http://bigbang:change_me@localhost\"")
	updateCmd.Flags().StringVarP(&repoPath, "path", "p", "", "The path for the repo to evaluate")
	updateCmd.MarkFlagRequired("url")
	updateCmd.MarkFlagRequired("path")
}
