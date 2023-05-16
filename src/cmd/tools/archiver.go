// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
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

var unarchiveAll bool

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

		if unarchiveAll {
			err := filepath.Walk(destinationPath, func(path string, info os.FileInfo, err error) error {
				if strings.HasSuffix(path, ".tar") {
					dst := filepath.Join(strings.TrimSuffix(path, ".tar"), "..")
					if info.Name() == "sboms.tar" {
						dst = strings.TrimSuffix(path, ".tar")
					}
					err := archiver.Unarchive(path, dst)
					if err != nil {
						return fmt.Errorf("failed to unarchive %s: %s", path, err.Error())
					}
					err = os.Remove(path)
					if err != nil {
						return fmt.Errorf("failed to remove %s: %s", path, err.Error())
					}
				}
				return nil
			})
			if err != nil {
				message.Fatalf(err, lang.CmdToolsArchiverUnarchiveAllErr)
			}
		}
	},
}

func init() {
	toolsCmd.AddCommand(archiverCmd)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)
	archiverDecompressCmd.Flags().BoolVar(&unarchiveAll, "unarchive-all", false, "Unarchive all tarballs in the archive")
}
