package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	loadCommand cliconfig.LoadValues

	loadDescription = "Loads the image from docker-archive stored on the local machine."
	_loadCommand    = &cobra.Command{
		Use:   "load [flags] [PATH]",
		Short: "Load an image from docker archive",
		Long:  loadDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			loadCommand.InputArgs = args
			loadCommand.GlobalFlags = MainGlobalOpts
			return loadCmd(&loadCommand)
		},
	}
)

func init() {
	loadCommand.Command = _loadCommand
	loadCommand.SetHelpTemplate(HelpTemplate())
	loadCommand.SetUsageTemplate(UsageTemplate())
	flags := loadCommand.Flags()
	flags.StringVarP(&loadCommand.Input, "input", "i", "/dev/stdin", "Read from archive file, default is STDIN")
	flags.BoolVarP(&loadCommand.Quiet, "quiet", "q", false, "Suppress the output")
	flags.StringVar(&loadCommand.SignaturePolicy, "signature-policy", "", "Pathname of signature policy file (not usually used)")

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

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	input := c.Input
	if runtime.Remote && len(input) == 0 {
		return errors.New("the remote client requires you to load via -i and a tarball")
	}
	if input == "/dev/stdin" {
		fi, err := os.Stdin.Stat()
		if err != nil {
			return err
		}
		// checking if loading from pipe
		if !fi.Mode().IsRegular() {
			outFile, err := ioutil.TempFile("/var/tmp", "podman")
			if err != nil {
				return errors.Errorf("error creating file %v", err)
			}
			defer os.Remove(outFile.Name())
			defer outFile.Close()

			inFile, err := os.OpenFile(input, 0, 0666)
			if err != nil {
				return errors.Errorf("error reading file %v", err)
			}
			defer inFile.Close()

			_, err = io.Copy(outFile, inFile)
			if err != nil {
				return errors.Errorf("error copying file %v", err)
			}

			input = outFile.Name()
		}
	}
	if err := parse.ValidateFileName(input); err != nil {
		return err
	}

	names, err := runtime.LoadImage(getContext(), imageName, c)
	if err != nil {
		return err
	}
	fmt.Println("Loaded image(s): " + names)
	return nil
}
