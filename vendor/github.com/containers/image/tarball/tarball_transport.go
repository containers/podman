package tarball

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/image/transports"
	"github.com/containers/image/types"
)

const (
	transportName = "tarball"
	separator     = ":"
)

var (
	// Transport implements the types.ImageTransport interface for "tarball:" images,
	// which are makeshift images constructed using one or more possibly-compressed tar
	// archives.
	Transport = &tarballTransport{}
)

type tarballTransport struct {
}

func (t *tarballTransport) Name() string {
	return transportName
}

func (t *tarballTransport) ParseReference(reference string) (types.ImageReference, error) {
	var stdin []byte
	var err error
	filenames := strings.Split(reference, separator)
	for _, filename := range filenames {
		if filename == "-" {
			stdin, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("error buffering stdin: %v", err)
			}
			continue
		}
		f, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("error opening %q: %v", filename, err)
		}
		f.Close()
	}
	ref := &tarballReference{
		transport: t,
		filenames: filenames,
		stdin:     stdin,
	}
	return ref, nil
}

func (t *tarballTransport) ValidatePolicyConfigurationScope(scope string) error {
	// See the explanation in daemonReference.PolicyConfigurationIdentity.
	return errors.New(`tarball: does not support any scopes except the default "" one`)
}

func init() {
	transports.Register(Transport)
}
