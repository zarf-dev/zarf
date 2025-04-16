// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/types"
)

type registryOptions struct {
	verbose  bool
	insecure bool
	ndlayers bool
	platform string
}

func newRegistryCommand() *cobra.Command {
	o := &registryOptions{
		verbose:  false,
		insecure: false,
		ndlayers: false,
		platform: "all",
	}

	// No package information is available so do not pass in a list of architectures
	craneOptions := []crane.Option{}

	cmd := &cobra.Command{
		Use:     "registry",
		Aliases: []string{"r", "crane"},
		Short:   lang.CmdToolsRegistryShort,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// TODO (@austinabro321) once the code in cmd is simplified, we should change this to respect
			// the log-format flag
			l := logger.Default()
			ctx := logger.WithContext(cmd.Context(), l)
			cmd.SetContext(ctx)
			// The crane options loading here comes from the rootCmd of crane
			craneOptions = append(craneOptions, crane.WithContext(cmd.Context()))
			// TODO(jonjohnsonjr): crane.Verbose option?
			if o.verbose {
				logs.Debug.SetOutput(os.Stderr)
			}
			if o.insecure {
				craneOptions = append(craneOptions, crane.Insecure)
			}
			if o.ndlayers {
				craneOptions = append(craneOptions, crane.WithNondistributable())
			}
			var err error
			var v1Platform *v1.Platform
			if o.platform != "all" {
				v1Platform, err = v1.ParsePlatform(o.platform)
				if err != nil {
					return fmt.Errorf("invalid platform %s: %w", o.platform, err)
				}
			}

			craneOptions = append(craneOptions, crane.WithPlatform(v1Platform))
			return nil
		},
	}

	cmd.AddCommand(newRegistryPruneCommand())
	cmd.AddCommand(newRegistryLoginCommand())
	cmd.AddCommand(newRegistryCopyCommand())
	cmd.AddCommand(newRegistryCatalogCommand())

	// TODO(soltysh): consider splitting craneOptions to be per command
	cmd.AddCommand(zarfCraneInternalWrapper(craneCmd.NewCmdList, &craneOptions, lang.CmdToolsRegistryListExample, 0))
	cmd.AddCommand(zarfCraneInternalWrapper(craneCmd.NewCmdPush, &craneOptions, lang.CmdToolsRegistryPushExample, 1))
	cmd.AddCommand(zarfCraneInternalWrapper(craneCmd.NewCmdPull, &craneOptions, lang.CmdToolsRegistryPullExample, 0))
	cmd.AddCommand(zarfCraneInternalWrapper(craneCmd.NewCmdDelete, &craneOptions, lang.CmdToolsRegistryDeleteExample, 0))
	cmd.AddCommand(zarfCraneInternalWrapper(craneCmd.NewCmdDigest, &craneOptions, lang.CmdToolsRegistryDigestExample, 0))

	cmd.AddCommand(craneCmd.NewCmdVersion())

	cmd.PersistentFlags().BoolVarP(&o.verbose, "verbose", "v", false, lang.CmdToolsRegistryFlagVerbose)
	cmd.PersistentFlags().BoolVar(&o.insecure, "insecure", false, lang.CmdToolsRegistryFlagInsecure)
	cmd.PersistentFlags().BoolVar(&o.ndlayers, "allow-nondistributable-artifacts", false, lang.CmdToolsRegistryFlagNonDist)
	cmd.PersistentFlags().StringVar(&o.platform, "platform", "all", lang.CmdToolsRegistryFlagPlatform)

	return cmd
}

func newRegistryLoginCommand() *cobra.Command {
	cmd := craneCmd.NewCmdAuthLogin()
	cmd.Example = ""
	err := cmd.MarkFlagRequired("username")
	if err != nil {
		logger.Default().Error("failed to mark username flag required", "error", err.Error())
	}
	cmd.MarkFlagsOneRequired("password", "password-stdin")
	return cmd
}

func newRegistryCopyCommand() *cobra.Command {
	// No package information is available so do not pass in a list of architectures
	craneOptions := []crane.Option{}
	cmd := craneCmd.NewCmdCopy(&craneOptions)
	return cmd
}

type registryCatalogOptions struct {
	craneOptions  []crane.Option
	originalRunFn func(cmd *cobra.Command, args []string) error
}

func newRegistryCatalogCommand() *cobra.Command {
	o := registryCatalogOptions{
		// No package information is available so do not pass in a list of architectures
		craneOptions: []crane.Option{},
	}

	cmd := craneCmd.NewCmdCatalog(&o.craneOptions)
	cmd.Example = lang.CmdToolsRegistryCatalogExample
	cmd.Args = nil

	o.originalRunFn = cmd.RunE
	cmd.RunE = o.run

	return cmd
}

func (o *registryCatalogOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	l := logger.From(cmd.Context())
	if len(args) > 0 {
		return o.originalRunFn(cmd, args)
	}

	l.Info("retrieving registry information from Zarf state")

	c, err := cluster.NewCluster(ctx)
	if err != nil {
		return err
	}

	zarfState, err := c.LoadState(ctx)
	if err != nil {
		return err
	}

	registryEndpoint, tunnel, err := c.ConnectToZarfRegistryEndpoint(ctx, zarfState.RegistryInfo)
	if err != nil {
		return err
	}

	// Add the correct authentication to the crane command options
	authOption := images.WithPullAuth(zarfState.RegistryInfo)
	o.craneOptions = append(o.craneOptions, authOption)

	if tunnel != nil {
		defer tunnel.Close()
		return tunnel.Wrap(func() error { return o.originalRunFn(cmd, []string{registryEndpoint}) })
	}

	return o.originalRunFn(cmd, []string{registryEndpoint})
}

type registryPruneOptions struct{}

func newRegistryPruneCommand() *cobra.Command {
	o := registryPruneOptions{}

	cmd := &cobra.Command{
		Use:     "prune",
		Aliases: []string{"p"},
		Short:   lang.CmdToolsRegistryPruneShort,
		RunE:    o.run,
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdToolsRegistryPruneFlagConfirm)

	return cmd
}

func (o *registryPruneOptions) run(cmd *cobra.Command, _ []string) error {
	// Try to connect to a Zarf initialized cluster
	c, err := cluster.NewCluster(cmd.Context())
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	l := logger.From(ctx)

	zarfState, err := c.LoadState(ctx)
	if err != nil {
		return err
	}

	zarfPackages, err := c.GetDeployedZarfPackages(ctx)
	if err != nil {
		return lang.ErrUnableToGetPackages
	}

	// Set up a tunnel to the registry if applicable
	registryEndpoint, tunnel, err := c.ConnectToZarfRegistryEndpoint(ctx, zarfState.RegistryInfo)
	if err != nil {
		return err
	}

	if tunnel != nil {
		l.Info("opening a tunnel to the Zarf registry", "local-endpoint", tunnel.Endpoint(), "cluster-address", zarfState.RegistryInfo.Address)
		defer tunnel.Close()
		return tunnel.Wrap(func() error { return doPruneImagesForPackages(ctx, zarfState, zarfPackages, registryEndpoint) })
	}

	return doPruneImagesForPackages(ctx, zarfState, zarfPackages, registryEndpoint)
}

func doPruneImagesForPackages(ctx context.Context, s *state.State, zarfPackages []types.DeployedPackage, registryEndpoint string) error {
	l := logger.From(ctx)
	authOption := images.WithPushAuth(s.RegistryInfo)

	l.Info("finding images to prune")

	// Determine which image digests are currently used by Zarf packages
	pkgImages := map[string]bool{}
	for _, pkg := range zarfPackages {
		deployedComponents := map[string]bool{}
		for _, depComponent := range pkg.DeployedComponents {
			deployedComponents[depComponent.Name] = true
		}

		for _, component := range pkg.Data.Components {
			if _, ok := deployedComponents[component.Name]; ok {
				for _, image := range component.Images {
					// We use the no checksum image since it will always exist and will share the same digest with other tags
					transformedImageNoCheck, err := transform.ImageTransformHostWithoutChecksum(registryEndpoint, image)
					if err != nil {
						return err
					}

					digest, err := crane.Digest(transformedImageNoCheck, authOption)
					if err != nil {
						return err
					}
					pkgImages[digest] = true
				}
			}
		}
	}

	// Find which images and tags are in the registry currently
	imageCatalog, err := crane.Catalog(registryEndpoint, authOption)
	if err != nil {
		return err
	}
	referenceToDigest := map[string]string{}
	for _, image := range imageCatalog {
		imageRef := fmt.Sprintf("%s/%s", registryEndpoint, image)
		tags, err := crane.ListTags(imageRef, authOption)
		if err != nil {
			return err
		}
		for _, tag := range tags {
			taggedImageRef := fmt.Sprintf("%s:%s", imageRef, tag)
			digest, err := crane.Digest(taggedImageRef, authOption)
			if err != nil {
				return err
			}
			referenceToDigest[taggedImageRef] = digest
		}
	}

	// Figure out which images are in the registry but not needed by packages
	imageDigestsToPrune := map[string]bool{}
	for digestRef, digest := range referenceToDigest {
		if _, ok := pkgImages[digest]; !ok {
			refInfo, err := transform.ParseImageRef(digestRef)
			if err != nil {
				return err
			}
			digestRef = fmt.Sprintf("%s@%s", refInfo.Name, digest)
			imageDigestsToPrune[digestRef] = true
		}
	}

	if len(imageDigestsToPrune) == 0 {
		l.Info("there are no images to prune")
		return nil
	}

	l.Info("the following image digests will be pruned from the registry:")
	for digestRef := range imageDigestsToPrune {
		l.Info(digestRef)
	}

	confirm := config.CommonOptions.Confirm
	if !confirm {
		prompt := &survey.Confirm{
			Message: "continue with image prune?",
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("confirm selection canceled: %w", err)
		}
	}
	if confirm {
		l.Info("pruning images")

		// Delete the digest references that are to be pruned
		for digestRef := range imageDigestsToPrune {
			err = crane.Delete(digestRef, authOption)
			if err != nil {
				return err
			}
			l.Debug("image pruned", "name", digestRef)
		}
	}
	return nil
}

// Wrap the original crane list with a zarf specific version
func zarfCraneInternalWrapper(commandToWrap func(*[]crane.Option) *cobra.Command, cranePlatformOptions *[]crane.Option, exampleText string, imageNameArgumentIndex int) *cobra.Command {
	wrappedCommand := commandToWrap(cranePlatformOptions)

	wrappedCommand.Example = exampleText
	wrappedCommand.Args = nil

	originalListFn := wrappedCommand.RunE

	wrappedCommand.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		l := logger.From(ctx)
		if len(args) < imageNameArgumentIndex+1 {
			return errors.New("not have enough arguments specified for this command")
		}

		// Try to connect to a Zarf initialized cluster otherwise then pass it down to crane.
		c, err := cluster.New(ctx)
		if err != nil {
			return originalListFn(cmd, args)
		}

		l.Info("retrieving registry information from Zarf state")

		s, err := c.LoadState(ctx)
		if err != nil {
			l.Warn("could not get Zarf state from Kubernetes cluster, continuing without state information", "error", err.Error())
			return originalListFn(cmd, args)
		}

		// Check to see if it matches the existing internal address.
		if !strings.HasPrefix(args[imageNameArgumentIndex], s.RegistryInfo.Address) {
			return originalListFn(cmd, args)
		}

		_, tunnel, err := c.ConnectToZarfRegistryEndpoint(ctx, s.RegistryInfo)
		if err != nil {
			return err
		}

		// Add the correct authentication to the crane command options
		authOption := images.WithPushAuth(s.RegistryInfo)
		*cranePlatformOptions = append(*cranePlatformOptions, authOption)

		if tunnel != nil {
			l.Info("opening a tunnel to the Zarf registry", "local-endpoint", tunnel.Endpoint(), "cluster-address", s.RegistryInfo.Address)

			defer tunnel.Close()

			givenAddress := fmt.Sprintf("%s/", s.RegistryInfo.Address)
			tunnelAddress := fmt.Sprintf("%s/", tunnel.Endpoint())
			args[imageNameArgumentIndex] = strings.Replace(args[imageNameArgumentIndex], givenAddress, tunnelAddress, 1)
			return tunnel.Wrap(func() error { return originalListFn(cmd, args) })
		}

		return originalListFn(cmd, args)
	}

	return wrappedCommand
}
