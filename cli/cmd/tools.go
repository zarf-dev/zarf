package cmd

import (
	"io/ioutil"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Various utilities shipped with Zarf",
}

var toolsTransformGitLinks = &cobra.Command{
	Use:   "patch-git-urls HOST FILE",
	Short: "Converts all .git URLs to the specified Zarf HOST and with the Zarf URL pattern in a given FILE",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host, fileName := args[0], args[1]

		// Read the contents of the given file
		content, err := ioutil.ReadFile(fileName)
		if err != nil {
			logrus.Fatal(err)
		}

		// Perform git url transformation via regex
		text := string(content)
		processedText := git.MutateGitUrlsInText(host, text)

		// Ask the user before this destructive action
		confirm := false
		prompt := &survey.Confirm{
			Message: "Overwrite the file " + fileName + " with these changes?",
		}
		survey.AskOne(prompt, &confirm)

		if confirm {
			// Overwrite the file
			err = ioutil.WriteFile(fileName, []byte(processedText), 0640)
			if err != nil {
				logrus.Fatal("Unable to write the changes back to the file")
			}
		}

	},
}

func init() {
	rootCmd.AddCommand(toolsCmd)
	toolsCmd.AddCommand(toolsTransformGitLinks)
}
