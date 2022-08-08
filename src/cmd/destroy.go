package cmd

import (
	"context"
	"errors"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"

	"github.com/defenseunicorns/zarf/src/internal/k8s"

	"github.com/spf13/cobra"
)

var confirmDestroy bool
var removeComponents bool

var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Aliases: []string{"d"},
	Short:   "Tear it all down, we'll miss you Zarf...",
	Long: "Tear down Zarf.\n\n" +
		"Deletes everything in the 'zarf' namespace within your connected k8s cluster.\n\n" +
		"If Zarf deployed your k8s cluster, this command will also tear your cluster down by " +
		"searching through /opt/zarf for any scripts that start with 'zarf-clean-' and executing them. " +
		"Since this is a cleanup operation, Zarf will not stop the teardown if one of the scripts produce " +
		"an error.\n\n" +
		"If Zarf did not deploy your k8s cluster, this command will delete the Zarf namespace, delete secrets " +
		"and labels that only Zarf cares about, and optionally uninstall components that Zarf deployed onto " +
		"the cluster. Since this is a cleanup operation, Zarf will not stop the uninstalls if one of the " +
		"resources produce an error while being deleted.",
	Run: func(cmd *cobra.Command, args []string) {
		// NOTE: If 'zarf init' failed to deploy the k3s component (or if we're looking at the wrong kubeconfig)
		//       there will be no zarf-state to load and the struct will be empty. In these cases, if we can find
		//       the scripts to remove k3s, we will still try to remove a locally installed k3s cluster
		state := k8s.LoadZarfState()

		// If Zarf deployed the cluster, burn it all down
		if state.ZarfAppliance || (state.Distro == "") {
			// Check if we have the scripts to destory everything
			fileInfo, err := os.Stat(config.ZarfCleanupScriptsPath)
			if errors.Is(err, os.ErrNotExist) || !fileInfo.IsDir() {
				message.Warnf("Unable to find the folder (%#v) which has the scripts to cleanup the cluster. Do you have the right kube-context?\n", config.ZarfCleanupScriptsPath)
				return
			}

			// Run all the scripts!
			pattern := regexp.MustCompile(`(?mi)zarf-clean-.+\.sh$`)
			scripts := utils.RecursiveFileList(config.ZarfCleanupScriptsPath, pattern)
			// Iterate over all matching zarf-clean scripts and exec them
			for _, script := range scripts {
				// Run the matched script
				_, _, err := utils.ExecCommandWithContext(context.TODO(), true, script)
				if errors.Is(err, os.ErrPermission) {
					message.Warnf("Got a 'permission denied' when trying to execute the script (%s). Are you the right user and/or do you have the right kube-context?\n", script)

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

			// If Zarf didn't deploy the cluster, only delete the ZarfNamespace
			k8s.DeleteZarfNamespace()

			// Remove zarf agent labels and secrets from namespaces Zarf doesn't manage
			k8s.StripZarfLabelsAndSecretsFromNamespaces()
		}
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, "REQUIRED. Confirm the destroy action to prevent accidental deletions")
	destroyCmd.Flags().BoolVar(&removeComponents, "remove-components", false, "Also remove any installed components outside the zarf namespace")
	_ = destroyCmd.MarkFlagRequired("confirm")
}
