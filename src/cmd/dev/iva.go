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

const (
	flagLayerCompression = "layer-compression"
	flagPlatformOS       = "platform-os"
	flagMaxLayer         = "max-layers"
)

type imageVolumeOptions struct {
	compression image.VolumeCompression
	os          image.PlatformOS
	output      string
	maxLayers   uint8
}

// NewImageVolumeCommand creates the command to build an image volume
// archive from a directory of files.
func NewImageVolumeCommand() *cobra.Command {
	o := &imageVolumeOptions{}

	cmd := &cobra.Command{
		Use:     lang.CmdDevImageVolumeArchiveUsage,
		Aliases: []string{"iva"},
		Short:   lang.CmdDevImageVolumeArchiveShort,
		Args:    cobra.ExactArgs(2),
		PreRunE: o.prerun,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP((*string)(&o.compression), flagLayerCompression, "c", string(image.VolumeCompressionGzip), lang.CmdDevImageVolumeArchiveFlagCompression)
	cmd.Flags().StringVarP((*string)(&o.os), flagPlatformOS, "o", string(image.PlatformOSLinux), lang.CmdDevImageVolumeArchiveFlagPlatformOS)
	cmd.Flags().StringVarP(&o.output, "output", "O", "", lang.CmdDevImageVolumeArchiveFlagOutput)
	cmd.Flags().Uint8VarP(&o.maxLayers, flagMaxLayer, "m", image.DefaultMaxLayers, lang.CmdDevImageVolumeArchiveFlagMaxLayer)

	if err := cmd.RegisterFlagCompletionFunc(flagLayerCompression, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return image.GetCobraCompression(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.From(cmd.Context()).Warn("failed to register out-complete", "error", err)
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc(flagPlatformOS, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return image.GetCobraPlatformOS(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.From(cmd.Context()).Warn("failed to register out-complete", "error", err)
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc(flagMaxLayer, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return image.GetCobraMaxLayers(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.From(cmd.Context()).Warn("failed to register out-complete", "error", err)
		panic(err)
	}

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

// run builds an image volume from args[0] (the source directory) tagged as
// args[1] (the image reference), then writes it to o.output as a
// Docker/OCI-compatible tar archive.
//
// cmd *cobra.Command, args []string.
// error.
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
	iv.MaxLayers = o.maxLayers

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
