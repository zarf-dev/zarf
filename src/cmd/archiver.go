// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/mholt/archives"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
)

// ldflags github.com/zarf-dev/zarf/src/cmd.archiverVersion=x.x.x
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
	cmd.AddCommand(newToolsVersionCmd("mholt/archiver", archiverVersion))

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

	if err := unarchive(context.Background(), sourceArchive, destinationPath); err != nil {
		return fmt.Errorf("unable to perform decompression: %w", err)
	}

	if !o.unarchiveAll {
		return nil
	}

	// for nested .tar files:
	return filepath.Walk(destinationPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".tar") {
			dst := filepath.Join(strings.TrimSuffix(p, ".tar"), "..")
			if info.Name() == layout.SBOMTar {
				dst = strings.TrimSuffix(p, ".tar")
			}
			if err := unarchive(context.Background(), p, dst); err != nil {
				return fmt.Errorf(lang.ErrUnarchive, p, err.Error())
			}
			if err := os.Remove(p); err != nil {
				return fmt.Errorf(lang.ErrRemoveFile, p, err.Error())
			}
		}
		return nil
	})
}

// unarchive opens src, identifies its format, and extracts into dst.
func unarchive(ctx context.Context, src, dst string) error {
	// 1) Open the archive file
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open archive %q: %w", src, err)
	}

	defer func() {
		err = errors.Join(err, file.Close())
	}()

	// 2) Identify format & get an input stream
	format, input, err := archives.Identify(ctx, src, file)
	if err != nil {
		return fmt.Errorf("unable to identify archive %q: %w", src, err)
	}

	// 3) Assert that it supports extraction
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction: %T", format)
	}

	// 4) Ensure dst exists
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("unable to create destination %q: %w", dst, err)
	}

	// 5) Define how each entry is written to disk
	handler := func(ctx context.Context, f archives.FileInfo) error { //nolint:revive
		target := filepath.Join(dst, f.NameInArchive)

		switch {
		case f.IsDir():
			// directory
			return os.MkdirAll(target, f.Mode())

		case f.LinkTarget != "":
			// symlink
			linkDest := filepath.Join(dst, f.LinkTarget)
			return os.Symlink(linkDest, target)

		default:
			// regular file
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, f.Mode())
			if err != nil {
				return err
			}

			defer func() {
				err = errors.Join(err, out.Close())
			}()

			in, err := f.Open()
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, in.Close())
			}()

			_, err = io.Copy(out, in)
			return err
		}
	}

	// 6) Perform extraction
	if err := extractor.Extract(ctx, input, handler); err != nil {
		return fmt.Errorf("unable to extract %q: %w", src, err)
	}
	return nil
}
