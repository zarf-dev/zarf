package cmd

import (
	cranecmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
)

// registryCmd represents the registry command
var registryCmd = &cobra.Command{
	Use: "registry",
	Short: "Collection of registry commands",
}

func init() {
	rootCmd.AddCommand(registryCmd)

	cranePlatformOptions := []crane.Option{
		crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"}),
	}
	registryCmd.AddCommand(cranecmd.NewCmdAuthLogin())
	registryCmd.AddCommand(cranecmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(cranecmd.NewCmdPush(&cranePlatformOptions))
}
