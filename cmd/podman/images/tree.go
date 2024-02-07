package images

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	treeDescription = "Print layer hierarchy of an image in a tree format"
	treeCmd         = &cobra.Command{
		Use:               "tree [options] IMAGE",
		Args:              cobra.ExactArgs(1),
		Short:             treeDescription,
		Long:              treeDescription,
		RunE:              tree,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           "podman image tree alpine:latest",
	}
	treeOpts entities.ImageTreeOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: treeCmd,
		Parent:  imageCmd,
	})
	treeCmd.Flags().BoolVar(&treeOpts.WhatRequires, "whatrequires", false, "Show all child images and layers of the specified image")
}

func tree(_ *cobra.Command, args []string) error {
	results, err := registry.ImageEngine().Tree(registry.Context(), args[0], treeOpts)
	if err != nil {
		return err
	}
	fmt.Println(results.Tree)
	return nil
}
