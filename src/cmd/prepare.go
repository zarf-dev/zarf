package cmd

import (
	"fmt"
	"os"

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
		content, err := os.ReadFile(fileName)
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
		if err := survey.AskOne(prompt, &confirm); err != nil {
			message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
		}

		if confirm {
			// Overwrite the file
			err = os.WriteFile(fileName, []byte(processedText), 0640)
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
	Use:     "find-images [PACKAGE]",
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

var prepareGenerateConfigFile = &cobra.Command{
	Use:     "generate-config [FILENAME]",
	Aliases: []string{"gc"},
	Args:    cobra.MaximumNArgs(1),
	Short:   "Generates a config file for Zarf",
	Long: "Generates a Zarf config file for controlling how the Zarf CLI operates. Optionally accepts a filename to write the config to.\n\n" +
		"The extension will determine the format of the config file, e.g. env-1.yaml, env-2.json, env-3.toml etc. \n" +
		"Accepted extensions are json, toml, yaml.\n\n" +
		"NOTE: This file must not already exist. If no filename is provided, the config will be written to the current working directory as zarf-config.toml.",
	Run: func(cmd *cobra.Command, args []string) {
		fileName := "zarf-config.toml"

		// If a filename was provided, use that
		if len(args) > 0 {
			fileName = args[0]
		}

		if err := v.SafeWriteConfigAs(fileName); err != nil {
			message.Fatalf(err, "Unable to write the config file %s, make sure the file doesn't already exist", fileName)
		}
	},
}

func init() {
	initViper()

	rootCmd.AddCommand(prepareCmd)
	prepareCmd.AddCommand(prepareTransformGitLinks)
	prepareCmd.AddCommand(prepareComputeFileSha256sum)
	prepareCmd.AddCommand(prepareFindImages)
	prepareCmd.AddCommand(prepareGenerateConfigFile)

	v.SetDefault("prepare.repo_chart_path", "")
	v.SetDefault("prepare.set", map[string]string{})

	prepareFindImages.Flags().StringVarP(&repoHelmChartPath, "repo-chart-path", "p", v.GetString("prepare.repo_chart_path"), `If git repos hold helm charts, often found with gitops tools, specify the chart path, e.g. "/" or "/chart"`)
	prepareFindImages.Flags().StringToStringVar(&config.CommonOptions.SetVariables, "set", v.GetStringMapString("prepare.set"), "Specify package variables to set on the command line (KEY=value)")
}
