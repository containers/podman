package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	treeCommand cliconfig.TreeValues

	treeDescription = "Prints layer hierarchy of an image in a tree format"
	_treeCommand    = &cobra.Command{
		Use:   "tree [flags] IMAGE",
		Short: treeDescription,
		Long:  treeDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			treeCommand.InputArgs = args
			treeCommand.GlobalFlags = MainGlobalOpts
			treeCommand.Remote = remoteclient
			return treeCmd(&treeCommand)
		},
		Example: "podman image tree alpine:latest",
	}
)

func init() {
	treeCommand.Command = _treeCommand
	treeCommand.SetUsageTemplate(UsageTemplate())
	treeCommand.Flags().BoolVar(&treeCommand.WhatRequires, "whatrequires", false, "Show all child images and layers of the specified image")
}

func treeCmd(c *cliconfig.TreeValues) error {
	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("an image name must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("you must provide at most 1 argument")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	tree, err := runtime.ImageTree(c.InputArgs[0], c.WhatRequires)
	if err != nil {
		return err
	}
	fmt.Print(tree)
	return nil
}
