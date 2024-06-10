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
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

// ldflags github.com/defenseunicorns/zarf/src/cmd/tools.archiverVersion=x.x.x
var archiverVersion string

var archiverCmd = &cobra.Command{
	Use:     "archiver",
	Aliases: []string{"a"},
	Short:   lang.CmdToolsArchiverShort,
	Version: archiverVersion,
}

var archiverCompressCmd = &cobra.Command{
	Use:     "compress SOURCES ARCHIVE",
	Aliases: []string{"c"},
	Short:   lang.CmdToolsArchiverCompressShort,
	Args:    cobra.MinimumNArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
		err := archiver.Archive(sourceFiles, destinationArchive)
		if err != nil {
			return fmt.Errorf("unable to perform compression: %w", err)
		}
		return err
	},
}

var unarchiveAll bool

var archiverDecompressCmd = &cobra.Command{
	Use:     "decompress ARCHIVE DESTINATION",
	Aliases: []string{"d"},
	Short:   lang.CmdToolsArchiverDecompressShort,
	Args:    cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		sourceArchive, destinationPath := args[0], args[1]
		err := archiver.Unarchive(sourceArchive, destinationPath)
		if err != nil {
			return fmt.Errorf("unable to perform decompression: %w", err)
		}
		if !unarchiveAll {
			return nil
		}
		err = filepath.Walk(destinationPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".tar") {
				dst := filepath.Join(strings.TrimSuffix(path, ".tar"), "..")
				// Unpack sboms.tar differently since it has a different folder structure than components
				if info.Name() == layout.SBOMTar {
					dst = strings.TrimSuffix(path, ".tar")
				}
				err := archiver.Unarchive(path, dst)
				if err != nil {
					return fmt.Errorf(lang.ErrUnarchive, path, err.Error())
				}
				err = os.Remove(path)
				if err != nil {
					return fmt.Errorf(lang.ErrRemoveFile, path, err.Error())
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("unable to unarchive all nested tarballs: %w", err)
		}
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(archiverCmd)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)
	archiverCmd.AddCommand(newVersionCmd("mholt/archiver", archiverVersion))
	archiverDecompressCmd.Flags().BoolVar(&unarchiveAll, "decompress-all", false, "Decompress all tarballs in the archive")
	archiverDecompressCmd.Flags().BoolVar(&unarchiveAll, "unarchive-all", false, "Unarchive all tarballs in the archive")
	archiverDecompressCmd.MarkFlagsMutuallyExclusive("decompress-all", "unarchive-all")
	archiverDecompressCmd.Flags().MarkHidden("decompress-all")
}
