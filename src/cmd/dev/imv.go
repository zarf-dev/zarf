// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package dev contains dev Zarf CLI commands.
package dev

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

type imageVolumeOptions struct {
	compression image.VolumeCompression
	os          image.PlatformOS
}

// NewImageVolumeCommand creates the command to build an image volume
// archive from a directory of files.
func NewImageVolumeCommand() *cobra.Command {
	o := &imageVolumeOptions{}

	cmd := &cobra.Command{
		Use:     "image-volume-archive [ DIRECTORY ] [ IMAGE REFERENCE ]",
		Aliases: []string{"iva"},
		Short:   lang.CmdInternalImageVolumeShort,
		Args:    cobra.ExactArgs(2),
		PreRunE: o.prerun,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP((*string)(&o.compression), "layer-compression", "c", string(image.VolumeCompressionGzip), "Compression algorthm used on the individual layers of the image volume")
	cmd.Flags().StringVarP((*string)(&o.os), "platform-os", "o", string(image.PlatformOSLinux), "Operating system of the image volume")

	return cmd
}

// prerun validates the compression format in the image volume options,
// defaulting to uncompressed if not specified.
//
// _ *cobra.Command, _ []string.
// error.
func (o *imageVolumeOptions) prerun(_ *cobra.Command, _ []string) error {
	return errors.Join(
		image.ValidateCompression(o.compression),
		image.ValidatePlatformOS(o.os),
		image.ValidatePlatformArch(image.PlatformArch(config.GetArch())),
	)
}

func (o *imageVolumeOptions) run(cmd *cobra.Command, args []string) error {
	dir := args[0]
	ref := args[1]

	iv, err := image.New(dir, string(o.os), config.GetArch())
	if err != nil {
		return err
	}
	defer func() {
		if cleanErr := iv.Clean(); cleanErr != nil {
			logger.From(cmd.Context()).Debug("failed to clean image volume workspace", "error", cleanErr)
		}
	}()

	iv.Compression = o.compression

	return iv.AddDirectory(cmd.Context(), dir, ref)
}
