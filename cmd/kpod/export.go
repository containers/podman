package main

import (
	"io"
	"os"

	"fmt"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type exportOptions struct {
	output    string
	container string
}

var (
	exportFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Write to a file, default is STDOUT",
			Value: "/dev/stdout",
		},
	}
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."
	exportCommand = cli.Command{
		Name:        "export",
		Usage:       "Export container's filesystem contents as a tar archive",
		Description: exportDescription,
		Flags:       exportFlags,
		Action:      exportCmd,
		ArgsUsage:   "CONTAINER",
	}
)

// exportCmd saves a container to a tarball on disk
func exportCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container id must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments given, need 1 at most.")
	}
	container := args[0]
	if err := validateFlags(c, exportFlags); err != nil {
		return err
	}

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return err
	}

	output := c.String("output")
	if output == "/dev/stdout" {
		file := os.Stdout
		if logrus.IsTerminal(file) {
			return errors.Errorf("refusing to export to terminal. Use -o flag or redirect")
		}
	}

	opts := exportOptions{
		output:    output,
		container: container,
	}

	return exportContainer(store, opts)
}

// exportContainer exports the contents of a container and saves it as
// a tarball on disk
func exportContainer(store storage.Store, opts exportOptions) error {
	mountPoint, err := store.Mount(opts.container, "")
	if err != nil {
		return errors.Wrapf(err, "error finding container %q", opts.container)
	}
	defer func() {
		if err := store.Unmount(opts.container); err != nil {
			fmt.Printf("error unmounting container %q: %v\n", opts.container, err)
		}
	}()

	input, err := archive.Tar(mountPoint, archive.Uncompressed)
	if err != nil {
		return errors.Wrapf(err, "error reading container directory %q", opts.container)
	}

	outFile, err := os.Create(opts.output)
	if err != nil {
		return errors.Wrapf(err, "error creating file %q", opts.output)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, input)
	return err
}
