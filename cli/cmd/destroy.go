package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"

	"github.com/spf13/cobra"
)

var confirmDestroy bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Tear it all down, we'll miss you Zarf...",
	Run: func(cmd *cobra.Command, args []string) {
		burn()
		_ = os.Remove(config.ZarfStatePath)
		pattern := regexp.MustCompile(`(?mi)zarf-clean-.+\.sh$`)
		scripts := utils.RecursiveFileList("/usr/local/bin", pattern)
		// Iterate over al matching zarf-clean scripts and exec them
		for _, script := range scripts {
			// Run the matched script
			_, _ = utils.ExecCommand(true, nil, script)
			// Try to remove the script, but ignore any errors
			_ = os.Remove(script)
		}
		burn()
	},
}

func burn() {
	fmt.Println("")
	for count := 0; count < 40; count++ {
		fmt.Print("🔥")
	}
	fmt.Println("")
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, "Confirm the destroy action")
	_ = destroyCmd.MarkFlagRequired("confirm")
}
