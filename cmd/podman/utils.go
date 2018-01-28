package main

import (
	"github.com/containers/storage"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

// Generate a new libpod runtime configured by command line options
func getRuntime(c *cli.Context) (*libpod.Runtime, error) {
	options := []libpod.RuntimeOption{}

	if c.GlobalIsSet("root") || c.GlobalIsSet("runroot") ||
		c.GlobalIsSet("storage-opt") {
		storageOpts := storage.DefaultStoreOptions

		if c.GlobalIsSet("root") {
			storageOpts.GraphRoot = c.GlobalString("root")
		}
		if c.GlobalIsSet("runroot") {
			storageOpts.RunRoot = c.GlobalString("runroot")
		}
		// TODO add CLI option to set graph driver
		if c.GlobalIsSet("storage-opt") {
			storageOpts.GraphDriverOptions = c.GlobalStringSlice("storage-opt")
		}

		options = append(options, libpod.WithStorageConfig(storageOpts))
	}

	// TODO CLI flags for image config?
	// TODO CLI flag for signature policy?

	if c.GlobalIsSet("runtime") {
		options = append(options, libpod.WithOCIRuntime(c.GlobalString("runtime")))
	}

	if c.GlobalIsSet("conmon") {
		options = append(options, libpod.WithConmonPath(c.GlobalString("conmon")))
	}

	// TODO flag to set CGroup manager?
	// TODO flag to set libpod static dir?
	// TODO flag to set libpod tmp dir?

	if c.GlobalIsSet("cni-config-dir") {
		options = append(options, libpod.WithCNIConfigDir(c.GlobalString("cni-config-dir")))
	}

	// TODO flag to set CNI plugins dir?

	return libpod.NewRuntime(options...)
}
