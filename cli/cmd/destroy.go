package cmd

import (
	"errors"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/helm"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/defenseunicorns/zarf/cli/types"

	"github.com/defenseunicorns/zarf/cli/internal/k8s"

	"github.com/spf13/cobra"
)

var confirmDestroy bool
var removeComponents bool

var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Aliases: []string{"d"},
	Short:   "Tear it all down, we'll miss you Zarf...",
	Run: func(cmd *cobra.Command, args []string) {
		// NOTE: If 'zarf init' failed to deploy the k3s component (or if we're looking at the wrong kubeconfig)
		//       there will be no zarf-state to load and the struct will be empty. In these cases, if we can find
		//       the scripts to remove k3s, we will still try to remove a locally installed k3s cluster
		state := k8s.LoadZarfState()

		// If Zarf deployed the cluster, burn it all down
		if state.ZarfAppliance || (state == types.ZarfState{}) {
			// Check if we have the scripts to destory everything
			fileInfo, err := os.Stat(config.ZarfCleanupScriptsPath)
			if errors.Is(err, os.ErrNotExist) || !fileInfo.IsDir() {
				message.Warnf("Unable to find the folder (%v) which has the scripts to cleanup the cluster. Do you have the right kube-context?\n", config.ZarfCleanupScriptsPath)
				return
			}

			// Run all the scripts!
			pattern := regexp.MustCompile(`(?mi)zarf-clean-.+\.sh$`)
			scripts := utils.RecursiveFileList(config.ZarfCleanupScriptsPath, pattern)
			// Iterate over al matching zarf-clean scripts and exec them
			for _, script := range scripts {
				// Run the matched script
				_, err := utils.ExecCommand(true, nil, script)
				if errors.Is(err, os.ErrPermission) {
					message.Warnf("Got a 'permission denied' when trying to execute the script (%v). Are you the right user and/or do you have the right kube-context?\n", script)

					// Don't remove scripts we can't execute so the user can try to manually run
					continue
				}

				// Try to remove the script, but ignore any errors
				_ = os.Remove(script)
			}
		} else {
			// Perform chart uninstallation
			helm.Destroy(removeComponents)

			// If Zarf didn't deploy the cluster, only delete the ZarfNamespace
			k8s.DeleteZarfNamespace()

			// Delete the zarf-registry secret in the default namespace
			defaultSecret, _ := k8s.GetSecret("default", "zarf-registry")
			k8s.DeleteSecret(defaultSecret)
		}
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, "Confirm the destroy action")
	destroyCmd.Flags().BoolVar(&removeComponents, "remove-components", false, "Also remove any installed components outside the zarf namespace")
	_ = destroyCmd.MarkFlagRequired("confirm")
}
