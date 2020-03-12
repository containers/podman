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
	rootCmd.PersistentFlags().BoolVarP(&dummyVersion, "version", "v", false, "Version of podman")
	rootCmd.PersistentFlags().BoolVarP(&registry.PodmanTunnel, "remote", "r", false, "Access service via SSH tunnel")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
