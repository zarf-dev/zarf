// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package dev contains dev Zarf CLI commands.
package dev

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/cmd/completion"
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
	flagOutput           = "output"
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
	// No shorthand for --platform-os: "-o" reads as output everywhere else.
	cmd.Flags().StringVar((*string)(&o.os), flagPlatformOS, string(image.PlatformOSLinux), lang.CmdDevImageVolumeArchiveFlagPlatformOS)
	cmd.Flags().StringVarP(&o.output, flagOutput, "o", "", lang.CmdDevImageVolumeArchiveFlagOutput)
	cmd.Flags().Uint8VarP(&o.maxLayers, flagMaxLayer, "m", image.DefaultMaxLayers, lang.CmdDevImageVolumeArchiveFlagMaxLayer)

	// Registration only fails when the flag doesn't exist or already has a
	// completion, both of which are wiring mistakes in the lines above rather
	// than anything a user can cause.
	completions := map[string]func() []string{
		flagLayerCompression: completion.ImageVolumeCompressions,
		flagPlatformOS:       completion.ImageVolumePlatformOSes,
		flagMaxLayer:         completion.ImageVolumeMaxLayers,
	}
	for flag, values := range completions {
		if err := cmd.RegisterFlagCompletionFunc(flag, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return values(), cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
			panic(err)
		}
	}

	return cmd
}

// prerun fills in the default output path from the image reference and
// validates every option that does not need the source directory to exist.
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
func (o *imageVolumeOptions) run(cmd *cobra.Command, args []string) error {
	dir := args[0]
	ref := args[1]
	l := logger.From(cmd.Context())

	// The default output path is derived from the reference and lands in the
	// working directory, so a second run of the same command overwrites the
	// first run's archive. Say so before spending the build on it.
	if _, err := os.Stat(o.output); err == nil {
		l.Warn("image volume archive already exists and will be replaced", "path", o.output)
	}

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
			l.Debug("failed to clean image volume workspace", "error", err)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			l.Debug("failed to remove staging directory", "error", err)
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
			l.Debug("failed to close image volume archive", "error", closeErr)
		}
	}()

	if err := iv.WriteTar(cmd.Context(), ref, out); err != nil {
		return err
	}

	l.Info("wrote image volume archive", "path", o.output)
	return nil
}
