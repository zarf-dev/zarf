// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"os"
	"regexp"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"

	"github.com/spf13/cobra"
)

var confirmDestroy bool
var removeComponents bool

var destroyCmd = &cobra.Command{
	Use:     "destroy --confirm",
	Aliases: []string{"d"},
	Short:   lang.CmdDestroyShort,
	Long:    lang.CmdDestroyLong,
	Run: func(_ *cobra.Command, _ []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cluster.DefaultTimeout)
		defer cancel()

		c, err := cluster.NewClusterWithWait(ctx)
		if err != nil {
			message.Fatalf(err, lang.ErrNoClusterConnection)
		}

		ctx, cancel = context.WithTimeout(context.Background(), cluster.DefaultTimeout)
		defer cancel()

		// NOTE: If 'zarf init' failed to deploy the k3s component (or if we're looking at the wrong kubeconfig)
		//       there will be no zarf-state to load and the struct will be empty. In these cases, if we can find
		//       the scripts to remove k3s, we will still try to remove a locally installed k3s cluster
		state, err := c.LoadZarfState(ctx)
		if err != nil {
			message.WarnErr(err, lang.ErrLoadState)
		}

		// If Zarf deployed the cluster, burn it all down
		if state.ZarfAppliance || (state.Distro == "") {
			// Check if we have the scripts to destroy everything
			fileInfo, err := os.Stat(config.ZarfCleanupScriptsPath)
			if errors.Is(err, os.ErrNotExist) || !fileInfo.IsDir() {
				message.Fatalf(lang.CmdDestroyErrNoScriptPath, config.ZarfCleanupScriptsPath)
			}

			// Run all the scripts!
			pattern := regexp.MustCompile(`(?mi)zarf-clean-.+\.sh$`)
			scripts, _ := helpers.RecursiveFileList(config.ZarfCleanupScriptsPath, pattern, true)
			// Iterate over all matching zarf-clean scripts and exec them
			for _, script := range scripts {
				// Run the matched script
				err := exec.CmdWithPrint(script)
				if errors.Is(err, os.ErrPermission) {
					message.Warnf(lang.CmdDestroyErrScriptPermissionDenied, script)

					// Don't remove scripts we can't execute so the user can try to manually run
					continue
				} else if err != nil {
					message.Debugf("Received error when trying to execute the script (%s): %#v", script, err)
				}

				// Try to remove the script, but ignore any errors
				_ = os.Remove(script)
			}
		} else {
			// Perform chart uninstallation
			helm.Destroy(removeComponents)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// If Zarf didn't deploy the cluster, only delete the ZarfNamespace
			if err := c.DeleteZarfNamespace(ctx); err != nil {
				message.Fatal(err, err.Error())
			}

			ctx, cancel = context.WithTimeout(context.Background(), cluster.DefaultTimeout)
			defer cancel()

			// Remove zarf agent labels and secrets from namespaces Zarf doesn't manage
			c.StripZarfLabelsAndSecretsFromNamespaces(ctx)
		}
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	// Still going to require a flag for destroy confirm, no viper oopsies here
	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, lang.CmdDestroyFlagConfirm)
	destroyCmd.Flags().BoolVar(&removeComponents, "remove-components", false, lang.CmdDestroyFlagRemoveComponents)
	_ = destroyCmd.MarkFlagRequired("confirm")
}
