package main

import (
	"fmt"
	"os"
	"path"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              path.Base(os.Args[0]),
	Long:             "Manage pods, containers and images",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	RunE:             registry.SubCommandExists,
	Version:          version.Version,
}

func init() {
	// Override default --help information of `--version` global flag}
	var dummyVersion bool
	// TODO had to disable shorthand -v for version due to -v rm with volume
	rootCmd.PersistentFlags().BoolVar(&dummyVersion, "version", false, "Version of Podman")
	rootCmd.PersistentFlags().StringVarP(&registry.EngineOpts.Uri, "remote", "r", "", "URL to access Podman service")
	rootCmd.PersistentFlags().StringSliceVar(&registry.EngineOpts.Identities, "identity", []string{}, "path to SSH identity file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
