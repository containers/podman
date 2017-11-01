package copy

import (
	"fmt"
	"io"

	"github.com/containers/image/signature"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

// createSignature creates a new signature of manifest at (identified by) dest using keyIdentity.
func createSignature(dest types.ImageDestination, manifest []byte, keyIdentity string, reportWriter io.Writer) ([]byte, error) {
	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return nil, errors.Wrap(err, "Error initializing GPG")
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return nil, errors.Wrap(err, "Signing not supported")
	}

	dockerReference := dest.Reference().DockerReference()
	if dockerReference == nil {
		return nil, errors.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(dest.Reference()))
	}

	fmt.Fprintf(reportWriter, "Signing manifest\n")
	newSig, err := signature.SignDockerManifest(manifest, dockerReference.String(), mech, keyIdentity)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating signature")
	}
	return newSig, nil
}
