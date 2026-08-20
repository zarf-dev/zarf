// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package dev contains dev Zarf CLI commands.
package dev

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci/archive"
	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

type imageVolumeOptions struct {
	compression image.VolumeCompression
	os          image.PlatformOS
	output      string
}

// NewImageVolumeCommand creates the command to build an image volume
// archive from a directory of files.
func NewImageVolumeCommand() *cobra.Command {
	o := &imageVolumeOptions{}

	cmd := &cobra.Command{
		Use:     "image-volume-archive [ DIRECTORY ] [ IMAGE-REFERENCE ]",
		Aliases: []string{"iva"},
		Short:   lang.CmdInternalImageVolumeShort,
		Args:    cobra.ExactArgs(2),
		PreRunE: o.prerun,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP((*string)(&o.compression), "layer-compression", "c", string(image.VolumeCompressionGzip), "Compression algorthm used on the individual layers of the image volume")
	cmd.Flags().StringVarP((*string)(&o.os), "platform-os", "o", string(image.PlatformOSLinux), "Operating system of the image volume")
	cmd.Flags().StringVarP(&o.output, "output", "O", "", "Path to write the resulting tar archive to (default derived from IMAGE-REFERENCE)")

	return cmd
}

// prerun validates the compression format in the image volume options,
// defaulting to uncompressed if not specified.
//
// _ *cobra.Command, args []string.
// error.
func (o *imageVolumeOptions) prerun(_ *cobra.Command, args []string) error {
	if o.output == "" {
		o.output = archive.ImageRefToTar(args[1])
	}
	return errors.Join(
		archive.ValidateFileEndsWithTar(o.output),
		image.ValidateCompression(o.compression),
		image.ValidatePlatformOS(o.os),
		image.ValidatePlatformArch(image.PlatformArch(config.GetArch())),
	)
}

func (o *imageVolumeOptions) run(cmd *cobra.Command, args []string) error {
	dir := args[0]
	ref := args[1]

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}

	iv, err := image.New(tmpDir, string(o.os), config.GetArch())
	if err != nil {
		return err
	}
	defer func() {
		if err := iv.Clean(); err != nil {
			logger.From(cmd.Context()).Debug("failed to clean image volume workspace", "error", err)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			logger.From(cmd.Context()).Debug("failed to remove staging directory", "error", err)
		}
	}()

	iv.Compression = o.compression

	if err := iv.AddDirectory(cmd.Context(), dir, ref); err != nil {
		return err
	}

	out, err := os.Create(o.output)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			logger.From(cmd.Context()).Debug("failed to close image volume archive", "error", closeErr)
		}
	}()

	if err := iv.WriteTar(cmd.Context(), ref, out); err != nil {
		return err
	}

	logger.From(cmd.Context()).Info("wrote image volume archive", "path", o.output)
	return nil
}
