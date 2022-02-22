package cmd

import (
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/cli/internal/helm"

	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/utils"

	"github.com/spf13/cobra"
)

var confirmDestroy bool
var removeComponents bool

var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Aliases: []string{"d"},
	Short:   "Tear it all down, we'll miss you Zarf...",
	Run: func(cmd *cobra.Command, args []string) {
		state := k8s.LoadZarfState()
		_ = os.Remove(".zarf-registry")

		if state.ZarfAppliance {
			// If Zarf deployed the cluster, burn it all down
			pattern := regexp.MustCompile(`(?mi)zarf-clean-.+\.sh$`)
			scripts := utils.RecursiveFileList("/usr/sbin", pattern)
			// Iterate over al matching zarf-clean scripts and exec them
			for _, script := range scripts {
				// Run the matched script
				_, _ = utils.ExecCommand(true, nil, script)
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
