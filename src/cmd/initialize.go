package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/defenseunicorns/zarf/src/internal/utils"

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
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)

		// Continue running package deploy for all components like any other package
		initPackageName := fmt.Sprintf("zarf-init-%s.tar.zst", config.GetArch())
		config.DeployOptions.PackagePath = initPackageName

		// Try to use an init-package in the executable directory if none exist in current working directory
		if utils.InvalidPath(config.DeployOptions.PackagePath) {
			executablePath, err := utils.GetFinalExecutablePath()
			if err != nil {
				message.Error(err, "Unable to get the directory where the zarf cli is located.")
			}

			executableDir := path.Dir(executablePath)
			config.DeployOptions.PackagePath = filepath.Join(executableDir, initPackageName)

			// If the init-package doesn't exist in the executable directory, suggest trying to download
			if utils.InvalidPath(config.DeployOptions.PackagePath) {

				if config.CommonOptions.Confirm {
					message.Fatalf(nil, "This command requires a zarf-init package, but one was not found on the local system.")
				}

				// If no CLI version exists (should only occur in dev or CI), try to get the latest release tag from Githhub
				if _, err := semver.StrictNewVersion(config.CLIVersion); err != nil {
					config.CLIVersion, err = utils.GetLatestReleaseTag(config.GithubProject)
					if err != nil {
						message.Fatal(err, "No CLI version found and unable to get the latest release tag for the zarf cli.")
					}
				}

				var confirmDownload bool
				url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", config.GithubProject, config.CLIVersion, initPackageName)

				// Give the user the choice to download the init-package and note that this does require an internet connection
				message.Question(fmt.Sprintf("It seems the init package could not be found locally, but can be downloaded from %s", url))

				message.Note("Note: This will require an internet connection.")

				// Prompt the user if --confirm not specified
				if !confirmDownload {
					prompt := &survey.Confirm{
						Message: "Do you want to download this init package?",
					}
					if err := survey.AskOne(prompt, &confirmDownload); err != nil {
						message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
					}
				}

				// If the user wants to download the init-package, download it
				if confirmDownload {
					utils.DownloadToFile(url, config.DeployOptions.PackagePath, "")
				} else {
					// Otherwise, exit and tell the user to manually download the init-package
					message.Warn("You must download the init package manually and place it in the current working directory")
					os.Exit(0)
				}
			}
		}

		// Run everything
		packager.Deploy()
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
