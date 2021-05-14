package cmd

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/spf13/cobra"
)

var cachePath = ".image-cache"
var targetPath = "oci-bundle"

var pullCmd = &cobra.Command{
	Use:   "pull IMAGE TARBALL",
	Short: "Pull remote images by reference and store their contents in a tarball",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		imageMap := map[string]v1.Image{}

		for _, src := range args {
			craneOptions := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})

			img, err := crane.Pull(src, craneOptions)
			if err != nil {
				return fmt.Errorf("pulling %s: %v", src, err)
			}
			img = cache.Image(img, cache.NewFilesystemCache(cachePath))

			imageMap[src] = img
		}

		if err := crane.MultiSaveOCI(imageMap, targetPath); err != nil {
			return fmt.Errorf("saving oci image layout %s: %v", targetPath, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
