package copy

import (
	"fmt"

	man "github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/pkg/errors"
)

// createSignature creates a new signature of manifest using keyIdentity.
func (c *copier) createSignature(manifest []byte, keyIdentity string) ([]byte, error) {
	if keyIdentity == "" {
		return c.createCosignSignature(manifest)
	}
	return c.createGPGSignature(manifest, keyIdentity)
}

// createGPGSignature creates a new GPG signature of manifest using keyIdentity.
func (c *copier) createGPGSignature(manifest []byte, keyIdentity string) ([]byte, error) {
	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return nil, errors.Wrap(err, "initializing GPG")
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
		return nil, errors.Wrap(err, "creating signature")
	}
	return newSig, nil
}

// createCosignSignature creates a new signature of manifest.
func (c *copier) createCosignSignature(manifest []byte) ([]byte, error) {
	mech, err := signature.NewSigstoreSigningMechanism()
	if err != nil {
		return nil, errors.Wrap(err, "initializing Sigstore")
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return nil, errors.Wrap(err, "signing is not supported")
	}

	dockerReference := c.dest.Reference().DockerReference()
	if dockerReference == nil {
		return nil, errors.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(c.dest.Reference()))
	}

	manifestDigest, err := man.Digest(manifest)
	if err != nil {
		return nil, err
	}

	fmt.Println("Generating certificate from Fulcio")
	if err := mech.GenerateCertificate(); err != nil {
		return nil, errors.Wrap(err, "getting key from Fulcio")
	}

	fmt.Println("Marshalling payload into JSON")
	cosignSignature := signature.NewCosignSignature(manifestDigest, dockerReference.String())
	sigPayload, err := cosignSignature.MarshalJSON()
	if err != nil {
		return nil, err
	}

	fmt.Println("Signing payload")
	signature, err := mech.Sign(sigPayload)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating signature for %s, %v", dockerReference.String(), manifestDigest)
	}

	fmt.Println("Sending entry to transparency log")
	tlogEntry, err := mech.Upload(signature, sigPayload)

	if err != nil {
		return nil, errors.Wrapf(err, "error uploading entry to transparency log for %s", dockerReference.String())
	}
	fmt.Println("Rekor entry successful. Index number: ", tlogEntry)

	return signature, nil
}
