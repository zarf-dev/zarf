// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
)

// ldflags github.com/zarf-dev/zarf/src/cmd/tools.archiverVersion=x.x.x
var archiverVersion string

func newArchiverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "archiver",
		Aliases: []string{"a"},
		Short:   lang.CmdToolsArchiverShort,
		Version: archiverVersion,
	}

	cmd.AddCommand(newArchiverCompressCommand())
	cmd.AddCommand(newArchiverDecompressCommand())
	cmd.AddCommand(newVersionCmd("mholt/archiver", archiverVersion))

	return cmd
}

type archiverCompressOptions struct{}

func newArchiverCompressCommand() *cobra.Command {
	o := archiverCompressOptions{}

	cmd := &cobra.Command{
		Use:     "compress SOURCES ARCHIVE",
		Aliases: []string{"c"},
		Short:   lang.CmdToolsArchiverCompressShort,
		Args:    cobra.MinimumNArgs(2),
		RunE:    o.run,
	}

	return cmd
}

func (o *archiverCompressOptions) run(_ *cobra.Command, args []string) error {
	sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
	err := archiver.Archive(sourceFiles, destinationArchive)
	if err != nil {
		return fmt.Errorf("unable to perform compression: %w", err)
	}
	return err
}

type archiverDecompressOptions struct {
	unarchiveAll bool
}

func newArchiverDecompressCommand() *cobra.Command {
	o := archiverDecompressOptions{}

	cmd := &cobra.Command{
		Use:     "decompress ARCHIVE DESTINATION",
		Aliases: []string{"d"},
		Short:   lang.CmdToolsArchiverDecompressShort,
		Args:    cobra.ExactArgs(2),
		RunE:    o.run,
	}

	cmd.Flags().BoolVar(&o.unarchiveAll, "decompress-all", false, "Decompress all tarballs in the archive")
	cmd.Flags().BoolVar(&o.unarchiveAll, "unarchive-all", false, "Unarchive all tarballs in the archive")
	cmd.MarkFlagsMutuallyExclusive("decompress-all", "unarchive-all")
	cmd.Flags().MarkHidden("decompress-all")

	return cmd
}

func (o *archiverDecompressOptions) run(_ *cobra.Command, args []string) error {
	sourceArchive, destinationPath := args[0], args[1]
	err := archiver.Unarchive(sourceArchive, destinationPath)
	if err != nil {
		return fmt.Errorf("unable to perform decompression: %w", err)
	}
	if !o.unarchiveAll {
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
}
