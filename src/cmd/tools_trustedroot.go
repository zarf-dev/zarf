// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

func newTrustedRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trusted-root",
		Short: lang.CmdToolsTrustedRootShort,
	}

	cmd.AddCommand(newTrustedRootFetchCommand())

	return cmd
}

type trustedRootFetchOptions struct {
	outputFile string
}

func newTrustedRootFetchCommand() *cobra.Command {
	o := &trustedRootFetchOptions{}

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: lang.CmdToolsTrustedRootFetchShort,
		Long:  lang.CmdToolsTrustedRootFetchLong,
		Args:  cobra.NoArgs,
		RunE:  o.run,
	}

	cmd.Flags().StringVarP(&o.outputFile, "output", "o", "trusted_root.json", lang.CmdToolsTrustedRootFetchFlagOutput)

	return cmd
}

func (o *trustedRootFetchOptions) run(cmd *cobra.Command, _ []string) error {
	return fetchTrustedRoot(cmd.Context(), o.outputFile)
}

func fetchTrustedRoot(ctx context.Context, outputFile string) error {
	l := logger.From(ctx)

	l.Debug("fetching Sigstore trusted root via TUF")

	tr, err := root.FetchTrustedRoot()
	if err != nil {
		return fmt.Errorf("failed to fetch trusted root: %w", err)
	}

	trJSON, err := tr.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal trusted root: %w", err)
	}

	if err := os.WriteFile(outputFile, trJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write trusted root to %s: %w", outputFile, err)
	}

	l.Info("trusted root written successfully", "path", outputFile, "size", len(trJSON))
	return nil
}
