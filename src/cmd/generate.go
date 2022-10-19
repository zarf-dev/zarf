package cmd

import (
	"errors"

	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/generator"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:     "generate COMMAND",
	Aliases: []string{"g"},
	Short:   "Zarf package generation wizard and commands",
}

var generateWizardCmd = &cobra.Command{
	Use:     "wizard",
	Aliases: []string{"w"},
	Short:   "Interactive wizard to assist with package creation",
	Long: "Starts an interactive sessions with zarf where the user will be quizzed survey\n" +
		"style to create a new zarf.yaml without needing prerequisite knowledge.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("Unimplemented")
		return err
	},
}
var generatePackageCmd = &cobra.Command{
	Use:     "package PACKAGE_NAME",
	Aliases: []string{"pkg"},
	Short:   "Create or modify a package",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		generatePackage, fileExists, computedDest := generator.GetPackageFromDestination(config.GenerateOptions.FilePath)

		if cmd.Flags().Changed("description") || !fileExists {
			generatePackage.Metadata.Description = config.GenerateOptions.PackageDescription
		}

		generatePackage.Metadata.Name = args[0]

		err := utils.WriteYaml(computedDest, generatePackage, 0644)
		if err != nil {
			message.Fatal(err, err.Error())
		}

	},
}

var generateComponentCmd = &cobra.Command{
	Use:     "component COMPONENT_NAME",
	Aliases: []string{"com"},
	Short:   "Create or modify a component",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("Unimplemented")
		return err
	},
}

var generateImageCmd = &cobra.Command{
	Use:     "image",
	Aliases: []string{"img"},
	Short:   "Add or modify an image in the list",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("Unimplemented")
		return err
	},
}

var generateConstantCmd = &cobra.Command{
	Use:     "constant",
	Aliases: []string{"con"},
	Short:   "Create or modify a constant",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("Unimplemented")
		return err
	},
}

var generateVariableCmd = &cobra.Command{
	Use:     "variable",
	Aliases: []string{"con"},
	Short:   "Create or modify a variable",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("Unimplemented")
		return err
	},
}

func init() {
	common.InitViper()

	rootCmd.AddCommand(generateCmd)
	generateCmd.AddCommand(generateWizardCmd)
	generateCmd.AddCommand(generatePackageCmd)
	generateCmd.AddCommand(generateComponentCmd)
	generateCmd.AddCommand(generateImageCmd)
	generateCmd.AddCommand(generateConstantCmd)
	generateCmd.AddCommand(generateVariableCmd)

	bindGenerateFlags()
	bindWizardFlags()
	bindSelfPackageGenerateFlags()
	bindComponentFlags()
	bindImageFlags()
	bindConstantFlags()
	bindVariableFlags()
}

func bindGenerateFlags() {
	generateCmd.PersistentFlags().StringVarP(&config.GenerateOptions.FilePath, "yaml-path", "f", "", "Path to the zarf yaml to generate or modify")
}

func bindWizardFlags() {
	generateWizardCmd.Flags()
}

func bindSelfPackageGenerateFlags() {
	packageFlags := generatePackageCmd.Flags()

	packageFlags.StringVarP(&config.GenerateOptions.PackageDescription, "description", "d", "", "The description of the package")
}

func bindComponentFlags() {
	generateComponentCmd.Flags()
}

func bindImageFlags() {
	generateImageCmd.Flags()
}

func bindConstantFlags() {
	generateConstantCmd.Flags()
}

func bindVariableFlags() {
	generateVariableCmd.Flags()
}
