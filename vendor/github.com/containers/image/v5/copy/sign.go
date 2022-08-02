package copy

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/private"
	internalsig "github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/signature/sigstore"
	"github.com/containers/image/v5/transports"
)

// sourceSignatures returns signatures from unparsedSource based on options,
// and verifies that they can be used (to avoid copying a large image when we
// can tell in advance that it would ultimately fail)
func (c *copier) sourceSignatures(ctx context.Context, unparsed private.UnparsedImage, options *Options,
	gettingSignaturesMessage, checkingDestMessage string) ([]internalsig.Signature, error) {
	var sigs []internalsig.Signature
	if options.RemoveSignatures {
		sigs = []internalsig.Signature{}
	} else {
		c.Printf("%s\n", gettingSignaturesMessage)
		s, err := unparsed.UntrustedSignatures(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading signatures: %w", err)
		}
		sigs = s
	}
	if len(sigs) != 0 {
		c.Printf("%s\n", checkingDestMessage)
		if err := c.dest.SupportsSignatures(ctx); err != nil {
			return nil, fmt.Errorf("Can not copy signatures to %s: %w", transports.ImageName(c.dest.Reference()), err)
		}
	}
	return sigs, nil
}

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
