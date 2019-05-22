package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	loadCommand cliconfig.LoadValues

	loadDescription = "Loads an image from a locally stored archive (tar file) into container storage."

	_loadCommand = &cobra.Command{
		Use:   "load [flags] [NAME[:TAG]]",
		Short: "Load an image from container archive",
		Long:  loadDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			loadCommand.InputArgs = args
			loadCommand.GlobalFlags = MainGlobalOpts
			loadCommand.Remote = remoteclient
			return loadCmd(&loadCommand)
		},
	}
)

func init() {
	loadCommand.Command = _loadCommand
	loadCommand.SetHelpTemplate(HelpTemplate())
	loadCommand.SetUsageTemplate(UsageTemplate())
	flags := loadCommand.Flags()
	flags.StringVarP(&loadCommand.Input, "input", "i", "", "Read from specified archive file (default: stdin)")
	flags.BoolVarP(&loadCommand.Quiet, "quiet", "q", false, "Suppress the output")
	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&loadCommand.SignaturePolicy, "signature-policy", "", "Pathname of signature policy file (not usually used)")
		flags.MarkHidden("signature-policy")
	}
}

// loadCmd gets the image/file to be loaded from the command line
// and calls loadImage to load the image to containers-storage
func loadCmd(c *cliconfig.LoadValues) error {

	args := c.InputArgs
	var imageName string

	if len(args) == 1 {
		imageName = args[0]
	}
	if len(args) > 1 {
		return errors.New("too many arguments. Requires exactly 1")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.Input) > 0 {
		if err := parse.ValidateFileName(c.Input); err != nil {
			return err
		}
	} else {
		if terminal.IsTerminal(int(os.Stdin.Fd())) {
			return errors.Errorf("cannot read from terminal. Use command-line redirection or the --input flag.")
		}
		outFile, err := ioutil.TempFile("/var/tmp", "podman")
		if err != nil {
			return errors.Errorf("error creating file %v", err)
		}
		defer os.Remove(outFile.Name())
		defer outFile.Close()

		_, err = io.Copy(outFile, os.Stdin)
		if err != nil {
			return errors.Errorf("error copying file %v", err)
		}

		c.Input = outFile.Name()
	}

	names, err := runtime.LoadImage(getContext(), imageName, c)
	if err != nil {
		return err
	}
	if len(imageName) > 0 {
		split := strings.Split(names, ",")
		newImage, err := runtime.NewImageFromLocal(split[0])
		if err != nil {
			return err
		}
		if err := newImage.TagImage(imageName); err != nil {
			return errors.Wrapf(err, "error adding '%s' to image %q", imageName, newImage.InputName)
		}
	}
	fmt.Println("Loaded image(s): " + names)
	return nil
}
