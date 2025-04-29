// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/archive"
)

// ldflags github.com/zarf-dev/zarf/src/cmd.archivesVersion=x.x.x
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
	// FIXME(mkcp): archivesVersion
	cmd.AddCommand(newToolsVersionCmd("mholt/archives", archiverVersion))

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

func (o *archiverCompressOptions) run(cmd *cobra.Command, args []string) error {
	sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
	return archive.Compress(cmd.Context(), sourceFiles, destinationArchive, archive.CompressOpts{})
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

func (o *archiverDecompressOptions) run(cmd *cobra.Command, args []string) error {
	return archive.Decompress(cmd.Context(), args[0], args[1], archive.DecompressOpts{
		UnarchiveAll: o.unarchiveAll,
	})
}
