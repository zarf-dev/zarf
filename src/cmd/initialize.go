package cmd

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"i"},
	Short:   "Prepares a k8s cluster for the deployment of Zarf packages",
	Long: "Injects a docker registry as well as other optional useful things (such as a git server " +
		"and a logging stack) into a k8s cluster under the 'zarf' namespace " +
		"to support future application deployments. \n" +

		"If you do not have a k8s cluster already configured, this command will give you " +
		"the ability to install a cluster locally.\n\n" +

		"This command looks for a zarf-init package in the local directory that the command was executed " +
		"from. If no package is found in the local directory and the Zarf CLI exists somewhere outside of " +
		"the current directory, Zarf will failover and attempt to find a zarf-init package in the directory " +
		"that the Zarf binary is located in.\n",

	Run: func(cmd *cobra.Command, args []string) {
		packager.InitializeCluster()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", "", "Specify the temporary directory to use for intermediate files")
	initCmd.Flags().StringVar(&config.DeployOptions.Components, "components", "", "Comma-separated list of components to install.")
	initCmd.Flags().StringVar(&config.DeployOptions.StorageClass, "storage-class", "", "Describe the StorageClass to be used")
	initCmd.Flags().StringVar(&config.DeployOptions.Secret, "secret", "", "Root secret value that is used to 'seed' other secrets")
	initCmd.Flags().StringVar(&config.DeployOptions.NodePort, "nodeport", "", "Nodeport to access the Zarf container registry. Between [30000-32767]")
}
