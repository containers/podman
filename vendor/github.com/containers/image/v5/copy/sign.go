package copy

import (
	"fmt"

	"github.com/containers/image/v5/docker/reference"
	internalsig "github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/signature/sigstore"
	"github.com/containers/image/v5/transports"
)

// createSignature creates a new signature of manifest using keyIdentity.
func (c *copier) createSignature(manifest []byte, keyIdentity string, passphrase string, identity reference.Named) (internalsig.Signature, error) {
	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return nil, fmt.Errorf("initializing GPG: %w", err)
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return nil, fmt.Errorf("Signing not supported: %w", err)
	}

	if identity != nil {
		if reference.IsNameOnly(identity) {
			return nil, fmt.Errorf("Sign identity must be a fully specified reference %s", identity)
		}
	} else {
		identity = c.dest.Reference().DockerReference()
		if identity == nil {
			return nil, fmt.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(c.dest.Reference()))
		}
	}

	c.Printf("Signing manifest using simple signing\n")
	newSig, err := signature.SignDockerManifestWithOptions(manifest, identity.String(), mech, keyIdentity, &signature.SignOptions{Passphrase: passphrase})
	if err != nil {
		return nil, fmt.Errorf("creating signature: %w", err)
	}
	return internalsig.SimpleSigningFromBlob(newSig), nil
}

// createSigstoreSignature creates a new sigstore signature of manifest using privateKeyFile and identity.
func (c *copier) createSigstoreSignature(manifest []byte, privateKeyFile string, passphrase []byte, identity reference.Named) (internalsig.Signature, error) {
	if identity != nil {
		if reference.IsNameOnly(identity) {
			return nil, fmt.Errorf("Sign identity must be a fully specified reference %s", identity.String())
		}
	} else {
		identity = c.dest.Reference().DockerReference()
		if identity == nil {
			return nil, fmt.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(c.dest.Reference()))
		}
	}

	c.Printf("Signing manifest using a sigstore signature\n")
	newSig, err := sigstore.SignDockerManifestWithPrivateKeyFileUnstable(manifest, identity, privateKeyFile, passphrase)
	if err != nil {
		return nil, fmt.Errorf("creating signature: %w", err)
	}
	return newSig, nil
}
