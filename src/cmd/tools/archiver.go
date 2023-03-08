// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

var archiverCmd = &cobra.Command{
	Use:     "archiver",
	Aliases: []string{"a"},
	Short:   lang.CmdToolsArchiverShort,
}

var archiverCompressCmd = &cobra.Command{
	Use:     "compress {SOURCES} {ARCHIVE}",
	Aliases: []string{"c"},
	Short:   lang.CmdToolsArchiverCompressShort,
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
		err := archiver.Archive(sourceFiles, destinationArchive)
		if err != nil {
			message.Fatal(err, lang.CmdToolsArchiverCompressErr)
		}
	},
}

var decompressLayers bool

var archiverDecompressCmd = &cobra.Command{
	Use:     "decompress {ARCHIVE} {DESTINATION}",
	Aliases: []string{"d"},
	Short:   lang.CmdToolsArchiverDecompressShort,
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceArchive, destinationPath := args[0], args[1]
		err := archiver.Unarchive(sourceArchive, destinationPath)
		if err != nil {
			message.Fatal(err, lang.CmdToolsArchiverDecompressErr)
		}

		// Decompress component layers in the destination path
		if decompressLayers {
			layersDir := filepath.Join(destinationPath, "components")

			files, err := os.ReadDir(layersDir)
			if err != nil {
				message.Fatalf(err, "failed to read the layers of components")
			}
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".tar") {
					if err := archiver.Unarchive(filepath.Join(layersDir, file.Name()), layersDir); err != nil {
						message.Fatalf(err, "failed to decompress the component layer")
					} else {
						// Without unarchive error, delete original tar.zst in component folder
						// This will leave the tar.zst if their is a failure for post mortem check
						_ = os.Remove(filepath.Join(layersDir, file.Name()))
					}
				}
			}
		}
	},
}

func init() {
	toolsCmd.AddCommand(archiverCmd)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)
	archiverDecompressCmd.Flags().BoolVar(&decompressLayers, "decompress-all", false, "Decompress all layers in the archive")
}
