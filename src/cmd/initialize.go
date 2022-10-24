package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"

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
		"that the Zarf binary is located in.\n\n\n\n" +

		"Example Usage:\n" +
		"# Initializing without any optional components:\nzarf init\n\n" +
		"# Initializing w/ Zarfs internal git server:\nzarf init --components=git-server\n\n" +
		"# Initializing w/ Zarfs internal git server and PLG stack:\nzarf init --components=git-server,logging\n\n" +
		"# Initializing w/ an internal registry but with a different nodeport:\nzarf init --nodeport=30333\n\n" +
		"# Initializing w/ an external registry:\nzarf init --registry-push-password={PASSWORD} --registry-push-username={USERNAME} --registry-url={URL}\n\n" +
		"# Initializing w/ an external git server:\nzarf init --git-push-password={PASSWORD} --git-push-username={USERNAME} --git-url={URL}\n\n",

	Run: func(cmd *cobra.Command, args []string) {
		zarfLogo := message.GetLogo()
		_, _ = fmt.Fprintln(os.Stderr, zarfLogo)

		err := validateInitFlags()
		if err != nil {
			message.Fatal(err, "Invalid command flags were provided.")
		}

		// Continue running package deploy for all components like any other package
		initPackageName := config.GetInitPackageName()
		config.DeployOptions.PackagePath = initPackageName

		// Try to use an init-package in the executable directory if none exist in current working directory
		if utils.InvalidPath(config.DeployOptions.PackagePath) {
			// Get the path to the executable
			if executablePath, err := utils.GetFinalExecutablePath(); err != nil {
				message.Errorf(err, "Unable to get the path to the executable")
			} else {
				executableDir := path.Dir(executablePath)
				config.DeployOptions.PackagePath = filepath.Join(executableDir, initPackageName)
			}

			// If the init-package doesn't exist in the executable directory, try the cache directory
			if err != nil || utils.InvalidPath(config.DeployOptions.PackagePath) {
				config.DeployOptions.PackagePath = filepath.Join(config.GetAbsCachePath(), initPackageName)

				// If the init-package doesn't exist in the cache directory, suggest downloading it
				if utils.InvalidPath(config.DeployOptions.PackagePath) {
					if err := downloadInitPackage(initPackageName); err != nil {
						message.Fatal(err, "Failed to download the init package")
					}
				}
			}
		}

		// Run everything
		pkg := packager.NewPackage()
		pkg.Deploy()
	},
}

func downloadInitPackage(initPackageName string) error {
	if config.CommonOptions.Confirm {
		return fmt.Errorf("this command requires a zarf-init package, but one was not found on the local system")
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
		return fmt.Errorf("you must download the init package manually and place it in the current working directory")
	}

	return nil
}

func validateInitFlags() error {
	// If 'git-url' is provided, make sure they provided values for the username and password of the push user
	if config.InitOptions.GitServer.Address != "" {
		if config.InitOptions.GitServer.PushUsername == "" || config.InitOptions.GitServer.PushPassword == "" {
			return fmt.Errorf("the 'git-push-username' and 'git-push-password' flags must be provided if the 'git-url' flag is provided")
		}
	}

	//If 'registry-url' is provided, make sure they provided values for the username and password of the push user
	if config.InitOptions.RegistryInfo.Address != "" {
		if config.InitOptions.RegistryInfo.PushUsername == "" || config.InitOptions.RegistryInfo.PushPassword == "" {
			return fmt.Errorf("the 'registry-push-username' and 'registry-push-password' flags must be provided if the 'registry-url' flag is provided ")
		}
	}
	return nil
}

func init() {
	initViper()

	rootCmd.AddCommand(initCmd)

	v.SetDefault(V_INIT_COMPONENTS, "")
	v.SetDefault(V_INIT_STORAGE_CLASS, "")

	v.SetDefault(V_INIT_GIT_URL, "")
	v.SetDefault(V_INIT_GIT_PUSH_USER, config.ZarfGitPushUser)
	v.SetDefault(V_INIT_GIT_PUSH_PASS, "")
	v.SetDefault(V_INIT_GIT_PULL_USER, "")
	v.SetDefault(V_INIT_GIT_PULL_PASS, "")

	v.SetDefault(V_INIT_REGISTRY_URL, "")
	v.SetDefault(V_INIT_REGISTRY_NODEPORT, 0)
	v.SetDefault(V_INIT_REGISTRY_SECRET, "")
	v.SetDefault(V_INIT_REGISTRY_PUSH_USER, config.ZarfRegistryPushUser)
	v.SetDefault(V_INIT_REGISTRY_PUSH_PASS, "")
	v.SetDefault(V_INIT_REGISTRY_PULL_USER, "")
	v.SetDefault(V_INIT_REGISTRY_PULL_PASS, "")

	// Continue to require --confirm flag for init command to avoid accidental deployments
	initCmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, "Confirm the install without prompting")
	initCmd.Flags().StringVar(&config.InitOptions.Components, "components", v.GetString(V_INIT_COMPONENTS), "Comma-separated list of components to install.")
	initCmd.Flags().StringVar(&config.InitOptions.StorageClass, "storage-class", v.GetString(V_INIT_STORAGE_CLASS), "Describe the StorageClass to be used")

	// Flags for using an external Git server
	initCmd.Flags().StringVar(&config.InitOptions.GitServer.Address, "git-url", v.GetString(V_INIT_GIT_URL), "External git server url to use for this Zarf cluster")
	initCmd.Flags().StringVar(&config.InitOptions.GitServer.PushUsername, "git-push-username", v.GetString(V_INIT_GIT_PUSH_USER), "Username to access to the git server Zarf is configured to use. User must be able to create repositories via 'git push'")
	initCmd.Flags().StringVar(&config.InitOptions.GitServer.PushPassword, "git-push-password", v.GetString(V_INIT_GIT_PUSH_PASS), "Password for the push-user to access the git server")
	initCmd.Flags().StringVar(&config.InitOptions.GitServer.PullUsername, "git-pull-username", v.GetString(V_INIT_GIT_PULL_USER), "Username for pull-only access to the git server")
	initCmd.Flags().StringVar(&config.InitOptions.GitServer.PullPassword, "git-pull-password", v.GetString(V_INIT_GIT_PULL_PASS), "Password for the pull-only user to access the git server")

	// Flags for using an external registry
	initCmd.Flags().StringVar(&config.InitOptions.RegistryInfo.Address, "registry-url", v.GetString(V_INIT_REGISTRY_URL), "External registry url address to use for this Zarf cluster")
	initCmd.Flags().IntVar(&config.InitOptions.RegistryInfo.NodePort, "nodeport", v.GetInt(V_INIT_REGISTRY_NODEPORT), "Nodeport to access a registry internal to the k8s cluster. Between [30000-32767]")
	initCmd.Flags().StringVar(&config.InitOptions.RegistryInfo.PushUsername, "registry-push-username", v.GetString(V_INIT_REGISTRY_PUSH_USER), "Username to access to the registry Zarf is configured to use")
	initCmd.Flags().StringVar(&config.InitOptions.RegistryInfo.PushPassword, "registry-push-password", v.GetString(V_INIT_REGISTRY_PUSH_PASS), "Password for the push-user to connect to the registry")
	initCmd.Flags().StringVar(&config.InitOptions.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(V_INIT_REGISTRY_PULL_USER), "Username for pull-only access to the registry")
	initCmd.Flags().StringVar(&config.InitOptions.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(V_INIT_REGISTRY_PULL_PASS), "Password for the pull-only user to access the registry")
	initCmd.Flags().StringVar(&config.InitOptions.RegistryInfo.Secret, "registry-secret", v.GetString(V_INIT_REGISTRY_SECRET), "Registry secret value")

	initCmd.Flags().SortFlags = true
}
