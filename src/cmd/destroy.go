package cmd

import (
	"github.com/defenseunicorns/zarf/src/internal/cluster"

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
		cluster.DestroyZarfCluster(removeComponents)
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().BoolVar(&confirmDestroy, "confirm", false, "REQUIRED. Confirm the destroy action to prevent accidental deletions")
	destroyCmd.Flags().BoolVar(&removeComponents, "remove-components", false, "Also remove any installed components outside the zarf namespace")
	_ = destroyCmd.MarkFlagRequired("confirm")
}
