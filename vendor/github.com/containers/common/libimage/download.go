package libimage

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

// tmpdir returns a path to a temporary directory.
func (r *Runtime) tmpdir() string {
	tmpdir := os.Getenv("TMPDIR")
	if tmpdir == "" {
		tmpdir = "/var/tmp"
	}

	return tmpdir
}

// downloadFromURL downloads an image in the format "https:/example.com/myimage.tar"
// and temporarily saves in it $TMPDIR/importxyz, which is deleted after the image is imported
func (r *Runtime) downloadFromURL(source string) (string, error) {
	fmt.Printf("Downloading from %q\n", source)

	outFile, err := ioutil.TempFile(r.tmpdir(), "import")
	if err != nil {
		return "", errors.Wrap(err, "error creating file")
	}
	defer outFile.Close()

	response, err := http.Get(source) // nolint:noctx
	if err != nil {
		return "", errors.Wrapf(err, "error downloading %q", source)
	}
	defer response.Body.Close()

	_, err = io.Copy(outFile, response.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error saving %s to %s", source, outFile.Name())
	}

	return outFile.Name(), nil
}
