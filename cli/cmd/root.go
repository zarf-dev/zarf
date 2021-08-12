package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "zarf",
	Short: "Small tool to bundle dependencies with K3s for airgapped deployments",
}

func Execute() {
	zarfLogo := getLogo()
	fmt.Fprintln(os.Stderr, zarfLogo)
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
