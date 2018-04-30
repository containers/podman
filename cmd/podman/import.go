package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod/image"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/urfave/cli"
)

var (
	importFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "change, c",
			Usage: "Apply the following possible instructions to the created image (default []): CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR",
		},
		cli.StringFlag{
			Name:  "message, m",
			Usage: "Set commit message for imported image",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output",
		},
	}
	importDescription = `Create a container image from the contents of the specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).
	 Note remote tar balls can be specified, via web address.
	 Optionally tag the image. You can specify the instructions using the --change option.
	`
	importCommand = cli.Command{
		Name:        "import",
		Usage:       "Import a tarball to create a filesystem image",
		Description: importDescription,
		Flags:       importFlags,
		Action:      importCmd,
		ArgsUsage:   "TARBALL [REFERENCE]",
	}
)

func importCmd(c *cli.Context) error {
	if err := validateFlags(c, importFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var (
		source    string
		reference string
		writer    io.Writer
	)

	args := c.Args()
	switch len(args) {
	case 0:
		return errors.Errorf("need to give the path to the tarball, or must specify a tarball of '-' for stdin")
	case 1:
		source = args[0]
	case 2:
		source = args[0]
		reference = args[1]
	default:
		return errors.Errorf("too many arguments. Usage TARBALL [REFERENCE]")
	}

	if err := validateFileName(source); err != nil {
		return err
	}

	changes := v1.ImageConfig{}
	if c.IsSet("change") {
		changes, err = util.GetImageConfig(c.StringSlice("change"))
		if err != nil {
			return errors.Wrapf(err, "error adding config changes to image %q", source)
		}
	}

	history := []v1.History{
		{Comment: c.String("message")},
	}

	config := v1.Image{
		Config:  changes,
		History: history,
	}

	writer = nil
	if !c.Bool("quiet") {
		writer = os.Stderr
	}

	// if source is a url, download it and save to a temp file
	u, err := url.ParseRequestURI(source)
	if err == nil && u.Scheme != "" {
		file, err := downloadFromURL(source)
		if err != nil {
			return err
		}
		defer os.Remove(file)
		source = file
	}

	newImage, err := runtime.ImageRuntime().Import(getContext(), source, reference, writer, image.SigningOptions{}, config)
	if err == nil {
		fmt.Println(newImage.ID())
	}
	return err
}

// donwloadFromURL downloads an image in the format "https:/example.com/myimage.tar"
// and tempoarily saves in it /var/tmp/importxyz, which is deleted after the image is imported
func downloadFromURL(source string) (string, error) {
	fmt.Printf("Downloading from %q\n", source)

	outFile, err := ioutil.TempFile("/var/tmp", "import")
	if err != nil {
		return "", errors.Wrap(err, "error creating file")
	}
	defer outFile.Close()

	response, err := http.Get(source)
	if err != nil {
		return "", errors.Wrapf(err, "error downloading %q", source)
	}
	defer response.Body.Close()

	_, err = io.Copy(outFile, response.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error saving %q to %q", source, outFile)
	}

	return outFile.Name(), nil
}
