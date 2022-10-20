package cmd
/*
import (
	"errors"

	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/generator"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
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
	Use: "package NAME",
	Aliases: []string{"pkg"},
	Short: "Generate an example package or from a given resource",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := validate.ValidatePackageName(args[0]); err != nil {
			message.Fatalf(err, "Invalid package name: %s", err.Error())
		}

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
	Use: "component COMPONENT_NAME [PROPERTY_PATH VALUE]",
	Aliases: []string{"com"},
	Short: "Create or modify a component",
	Long: "Command for creating or modifying components. Creating a component is as simple \n" +
	  "as providing just a name. To modify properties of a component a yaml path is required. \n" +
	  "If that path includes an array of objects then the selector flag is necessary to pick a \n" +
	  "specific object from the list. Properties that require a selector are files, charts, \n" +
	  "manifests, images, repos, and dataInjections. The selector must only match one of the \n" +
	  "given object type, if a selector doesn't match any objects a new one will be created \n" +
	  "with the given data and selector.",
	Args: func(cmd *cobra.Command, args []string) error {
		argLength := len(args)
		if argLength == 0 {
			return errors.New("a component name must be provided")
		} else if argLength != 1 && argLength != 3 {
			return errors.New("a property_path must be accompanied by a value to set it to")
		} else {
			return nil
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		generatePackage, fileExists, computedDest := generator.GetPackageFromDestination(config.GenerateOptions.FilePath)

		if !fileExists {
			message.Fatal("", "The given file must exist to be able to create/modify components")
		}

		var generateComponentP *types.ZarfComponent

		for idx, component := range generatePackage.Components {
			if component.Name == args[0] {
				generateComponentP = &generatePackage.Components[idx]
				break
			}
		}

		// Pointer is nil so component doesn't exist
		if generateComponentP == nil {
			newComponent := types.ZarfComponent{Name: args[0]}

			// Check if more than a component name was specified
			if len(args) > 1 {
				message.Fatal(errors.New("Unimplemented"), "Unimplemented")
			} else {
				generatePackage.Components = append(generatePackage.Components, newComponent)
			}

			// Pointer has value so component exists
		} else {

			// Check if more than a component name was specified
			if len(args) > 1 {
				message.Fatal(errors.New("Unimplemented"), "Unimplemented")
			} else {
				message.Info("Component already exists, exiting...")
			}
		}

		err := utils.WriteYaml(computedDest, generatePackage, 0644)
		if err != nil {
			message.Fatal(err, err.Error())
		}
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
	componentFlags := generateComponentCmd.Flags()

	componentFlags.StringVarP(&config.GenerateOptions.PropertySelector, "selector", "s", "", "The key value pair for picking an object from a list. Format is KEY:VALUE")
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
*/
