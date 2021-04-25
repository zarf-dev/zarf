package cmd

import (
	"fmt"
	"shift/internal/utils"

	"github.com/spf13/cobra"
)

var filePath string

// checksumCmd represents the checksum command
var checksumCmd = &cobra.Command{
	Use:   "checksum",
	Short: "Compute the SHA256 hash of the given file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(utils.GetSha256(filePath))
	},
}

func init() {
	rootCmd.AddCommand(checksumCmd)
	checksumCmd.Flags().StringVarP(&filePath, "file", "f", "", "The file path to generate a checksum for")
	checksumCmd.MarkFlagRequired("file")
}
