package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/spf13/cobra"
)

var repoHelmChartPath string
var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Tools to help prepare assets for packaging",
}

var prepareTransformGitLinks = &cobra.Command{
	Use:     "patch-git [HOST] [FILE]",
	Aliases: []string{"p"},
	Short: "Converts all .git URLs to the specified Zarf HOST and with the Zarf URL pattern in a given FILE.  NOTE: \n" +
		"This should only be used for manifests that are not mutated by the Zarf Agent Mutating Webhook.",
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host, fileName := args[0], args[1]

		// Read the contents of the given file
		content, err := ioutil.ReadFile(fileName)
		if err != nil {
			message.Fatalf(err, "Unable to read the file %s", fileName)
		}

		// Perform git url transformation via regex
		text := string(content)
		processedText := git.MutateGitUrlsInText(host, text, config.InitOptions.GitServerInfo.GitPushUsername)

		// Ask the user before this destructive action
		confirm := false
		prompt := &survey.Confirm{
			Message: "Overwrite the file " + fileName + " with these changes?",
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
		}

		if confirm {
			// Overwrite the file
			err = ioutil.WriteFile(fileName, []byte(processedText), 0640)
			if err != nil {
				message.Fatal(err, "Unable to write the changes back to the file")
			}
		}

	},
}

var prepareComputeFileSha256sum = &cobra.Command{
	Use:     "sha256sum [FILE|URL]",
	Aliases: []string{"s"},
	Short:   "Generate a SHA256SUM for the given file",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fileName := args[0]
		hash, err := utils.GetSha256Sum(fileName)
		if err != nil {
			message.Fatal(err, "Unable to compute the hash")
		} else {
			fmt.Println(hash)
		}
	},
}

var prepareFindImages = &cobra.Command{
	Use:     "find-images",
	Aliases: []string{"f"},
	Args:    cobra.MaximumNArgs(1),
	Short:   "Evaluates components in a zarf file to identify images specified in their helm charts and manifests",
	Long: "Evaluates components in a zarf file to identify images specified in their helm charts and manifests.\n\n" +
		"Components that have repos that host helm charts can be processed by providing the --repo-chart-path.",
	Run: func(cmd *cobra.Command, args []string) {
		var baseDir string

		// If a directory was provided, use that as the base directory
		if len(args) > 0 {
			baseDir = args[0]
		}

		packager.FindImages(baseDir, repoHelmChartPath)
	},
}

func init() {
	rootCmd.AddCommand(prepareCmd)
	prepareCmd.AddCommand(prepareTransformGitLinks)
	prepareCmd.AddCommand(prepareComputeFileSha256sum)
	prepareCmd.AddCommand(prepareFindImages)

	prepareFindImages.Flags().StringVarP(&repoHelmChartPath, "repo-chart-path", "p", "", `If git repos hold helm charts, often found with gitops tools, specify the chart path, e.g. "/" or "/chart"`)
	prepareFindImages.Flags().StringVar(&config.CommonOptions.TempDirectory, "tmpdir", "", "Specify the temporary directory to use for intermediate files")
	prepareFindImages.Flags().StringToStringVar(&config.CommonOptions.SetVariables, "set", map[string]string{}, "Specify package variables to set on the command line (KEY=value)")

	prepareTransformGitLinks.Flags().StringVar(&config.InitOptions.GitServerInfo.GitPushUsername, "git-username", config.ZarfGitPushUser, "Username for the git account that the repos are created under.")
}
