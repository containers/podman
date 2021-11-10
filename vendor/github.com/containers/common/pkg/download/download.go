package download

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// FromURL downloads the specified source to a file in tmpdir (OS defaults if
// empty).
func FromURL(tmpdir, source string) (string, error) {
	tmp, err := ioutil.TempFile(tmpdir, "")
	if err != nil {
		return "", fmt.Errorf("creating temporary download file: %w", err)
	}
	defer tmp.Close()

	response, err := http.Get(source) // nolint:noctx
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", source, err)
	}
	defer response.Body.Close()

	_, err = io.Copy(tmp, response.Body)
	if err != nil {
		return "", fmt.Errorf("copying %s to %s: %w", source, tmp.Name(), err)
	}

	return tmp.Name(), nil
}
