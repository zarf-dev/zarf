// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package inter contains internal Zarf CLI commands.
package inter

import (
	"errors"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

type internalImageVolumeOptions struct {
	compression image.ImageVolumeCompression
	os          image.ImagePlatformOS
	arch        image.ImagePlatformArch
}

// NewInternalImageVolumeCommand creates the command to build an image volume
// archive from a directory of files.
func NewInternalImageVolumeCommand() *cobra.Command {
	o := &internalImageVolumeOptions{}

	cmd := &cobra.Command{
		Use:     "image-volume-archive [ DIRECTORY ] [ IMAGE REFERENCE ]",
		Aliases: []string{"iva"},
		Short:   lang.CmdInternalImageVolumeShort,
		Args:    cobra.ExactArgs(2),
		PreRunE: o.prerun,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP((*string)(&o.compression), "layer-compression", "c", string(image.ImageVolumeCompressionGzip), "Compression algorthm used on the individual layers of the image volume")
	cmd.Flags().StringVarP((*string)(&o.os), "platform-os", "O", string(image.ImagePlatformOSLinux), "Operating system of the image volume")
	cmd.Flags().StringVarP((*string)(&o.arch), "platform-arch", "A", runtime.GOARCH, "The architecture of the image volume")

	return cmd
}

// prerun validates the compression format in the internal image volume options,
// defaulting to uncompressed if not specified.
//
// _ *cobra.Command, _ []string.
// error.
func (o *internalImageVolumeOptions) prerun(_ *cobra.Command, _ []string) error {
	return errors.Join(
		image.ValidateCompression(o.compression),
		image.ValidatePlatformOS(o.os),
		image.ValidatePlatformArch(o.arch),
	)
}

func (o *internalImageVolumeOptions) run(cmd *cobra.Command, args []string) error {
	dir := args[0]
	ref := args[1]

	iv, err := image.NewImageVolume(dir, "linux", "amd64")
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
