// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"

	"github.com/spf13/cobra"
)

var confirmDestroy bool
var removeComponents bool

var destroyCmd = &cobra.Command{
	Use:     "destroy --confirm",
	Aliases: []string{"d"},
	Short:   lang.CmdDestroyShort,
	Long:    lang.CmdDestroyLong,
	// TODO(mkcp): refactor and de-nest this function.
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		timeoutCtx, cancel := context.WithTimeout(cmd.Context(), cluster.DefaultTimeout)
		defer cancel()
		c, err := cluster.NewClusterWithWait(timeoutCtx)
		if err != nil {
			return err
		}

		// NOTE: If 'zarf init' failed to deploy the k3s component (or if we're looking at the wrong kubeconfig)
		//       there will be no zarf-state to load and the struct will be empty. In these cases, if we can find
		//       the scripts to remove k3s, we will still try to remove a locally installed k3s cluster
		state, err := c.LoadZarfState(ctx)
		if err != nil {
			message.WarnErr(err, err.Error())
		}

		// If Zarf deployed the cluster, burn it all down
		if state.ZarfAppliance || (state.Distro == "") {
			// Check if we have the scripts to destroy everything
			fileInfo, err := os.Stat(config.ZarfCleanupScriptsPath)
			if errors.Is(err, os.ErrNotExist) || !fileInfo.IsDir() {
				return fmt.Errorf("unable to find the folder %s which has the scripts to cleanup the cluster. Please double-check you have the right kube-context", config.ZarfCleanupScriptsPath)
			}

			// Run all the scripts!
			pattern := regexp.MustCompile(`(?mi)zarf-clean-.+\.sh$`)
			scripts, err := helpers.RecursiveFileList(config.ZarfCleanupScriptsPath, pattern, true)
			if err != nil {
				return err
			}
			// Iterate over all matching zarf-clean scripts and exec them
			for _, script := range scripts {
				// Run the matched script
				err := exec.CmdWithPrint(script)
				if errors.Is(err, os.ErrPermission) {
					message.Warnf(lang.CmdDestroyErrScriptPermissionDenied, script)

					// Don't remove scripts we can't execute so the user can try to manually run
					continue
				} else if err != nil {
					return fmt.Errorf("received an error when executing the script %s: %w", script, err)
				}

				// Try to remove the script, but ignore any errors and debug log them
				err = os.Remove(script)
				if err != nil {
					slog.Debug("Unable to remove script", "script", script, "error", err)
				}
			}
		} else {
			// Perform chart uninstallation
			helm.Destroy(removeComponents)

			// If Zarf didn't deploy the cluster, only delete the ZarfNamespace
			if err := c.DeleteZarfNamespace(ctx); err != nil {
				return err
			}

			// Remove zarf agent labels and secrets from namespaces Zarf doesn't manage
			c.StripZarfLabelsAndSecretsFromNamespaces(ctx)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	// Still going to require a flag for destroy confirm, no viper oopsies here
	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, lang.CmdDestroyFlagConfirm)
	destroyCmd.Flags().BoolVar(&removeComponents, "remove-components", false, lang.CmdDestroyFlagRemoveComponents)
	_ = destroyCmd.MarkFlagRequired("confirm")
}
