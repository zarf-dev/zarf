package cmd

import (
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

// destroyCmd represents the initialize command
var archiverCmd = &cobra.Command{
	Use:   "archiver",
	Short: "Compress/Decompress tools",
}

var archiverCompressCmd = &cobra.Command{
	Use:   "compress SOURCES ARCHIVE",
	Short: "Compress a collection of sources based off of the destination file extension",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
		archiver.Archive(sourceFiles, destinationArchive)

	},
}

var archiverDecompressCmd = &cobra.Command{
	Use:   "decompress ARCHIVE DESTINATION",
	Short: "Decompress an archive to a specified location.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceArchive, destinationPath := args[0], args[1]
		archiver.Unarchive(sourceArchive, destinationPath)
	},
}

func init() {
	rootCmd.AddCommand(archiverCmd)
	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)
}
