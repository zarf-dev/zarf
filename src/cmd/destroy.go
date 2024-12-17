// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"

	"github.com/spf13/cobra"
)

// DestroyOptions holds the command-line options for 'destroy' sub-command.
type DestroyOptions struct {
	confirmDestroy   bool
	removeComponents bool
}

// NewDestroyCommand creates the `destroy` sub-command.
func NewDestroyCommand() *cobra.Command {
	o := DestroyOptions{}
	cmd := &cobra.Command{
		Use:     "destroy --confirm",
		Aliases: []string{"d"},
		Short:   lang.CmdDestroyShort,
		Long:    lang.CmdDestroyLong,
		RunE:    o.Run,
	}

	// Still going to require a flag for destroy confirm, no viper oopsies here
	cmd.Flags().BoolVar(&o.confirmDestroy, "confirm", false, lang.CmdDestroyFlagConfirm)
	cmd.Flags().BoolVar(&o.removeComponents, "remove-components", false, lang.CmdDestroyFlagRemoveComponents)
	_ = cmd.MarkFlagRequired("confirm")

	return cmd
}

// Run performs the execution of 'destroy' sub-command.
func (o *DestroyOptions) Run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	l := logger.From(ctx)

	timeoutCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
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
		l.Warn(err.Error())
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
				l.Warn("received 'permission denied' when trying to execute script. Please double-check you have the correct kube-context.", "script", script)

				// Don't remove scripts we can't execute so the user can try to manually run
				continue
			} else if err != nil {
				return fmt.Errorf("received an error when executing the script %s: %w", script, err)
			}

			// Try to remove the script, but ignore any errors and debug log them
			err = os.Remove(script)
			if err != nil {
				message.WarnErr(err, fmt.Sprintf("Unable to remove script. script=%s", script))
				l.Warn("unable to remove script", "script", script, "error", err.Error())
			}
		}
	} else {
		// Perform chart uninstallation
		helm.Destroy(ctx, o.removeComponents)

		// If Zarf didn't deploy the cluster, only delete the ZarfNamespace
		if err := c.DeleteZarfNamespace(ctx); err != nil {
			return err
		}

		// Remove zarf agent labels and secrets from namespaces Zarf doesn't manage
		c.StripZarfLabelsAndSecretsFromNamespaces(ctx)
	}
	return nil
}
