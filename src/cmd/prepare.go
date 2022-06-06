package cmd

import (
	"fmt"
	"io/ioutil"

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
	Use:   "patch-git [HOST] [FILE]",
	Short: "Converts all .git URLs to the specified Zarf HOST and with the Zarf URL pattern in a given FILE",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host, fileName := args[0], args[1]

		// Read the contents of the given file
		content, err := ioutil.ReadFile(fileName)
		if err != nil {
			message.Fatalf(err, "Unable to read the file %s", fileName)
		}

		// Perform git url transformation via regex
		text := string(content)
		processedText := git.MutateGitUrlsInText(host, text)

		// Ask the user before this destructive action
		confirm := false
		prompt := &survey.Confirm{
			Message: "Overwrite the file " + fileName + " with these changes?",
		}
		_ = survey.AskOne(prompt, &confirm)

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
	Use:   "sha256sum [FILE|URL]",
	Short: "Generate a SHA256SUM for the given file",
	Args:  cobra.ExactArgs(1),
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
	Aliases: []string{"prep"},
	Short: "Evaluates components in a zarf file to identify images specified in their helm charts and manifests.\n" +
		"Requires the `--repo-chart-path` flag to be provided.",
	Run: func(cmd *cobra.Command, args []string) {
		packager.FindImages(repoHelmChartPath)
	},
}

func init() {
	rootCmd.AddCommand(prepareCmd)
	prepareCmd.AddCommand(prepareTransformGitLinks)
	prepareCmd.AddCommand(prepareComputeFileSha256sum)
	prepareCmd.AddCommand(prepareFindImages)

	prepareFindImages.Flags().StringVarP(&repoHelmChartPath, "repo-chart-path", "p", "", `If git repos hold helm charts, often found with gitops tools, specify the chart path, e.g. "/" or "/chart"`)

}
