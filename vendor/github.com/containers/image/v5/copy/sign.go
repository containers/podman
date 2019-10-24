package copy

import (
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/pkg/errors"
)

// createSignature creates a new signature of manifest using keyIdentity.
func (c *copier) createSignature(manifest []byte, keyIdentity string) ([]byte, error) {
	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return nil, errors.Wrap(err, "Error initializing GPG")
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return nil, errors.Wrap(err, "Signing not supported")
	}

	dockerReference := c.dest.Reference().DockerReference()
	if dockerReference == nil {
		return nil, errors.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(c.dest.Reference()))
	}

	c.Printf("Signing manifest\n")
	newSig, err := signature.SignDockerManifest(manifest, dockerReference.String(), mech, keyIdentity)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating signature")
	}
	return newSig, nil
}
