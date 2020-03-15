// Note: Consider the API unstable until the code supports at least three different image formats or transports.

package signature

import (
	"fmt"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/opencontainers/go-digest"
)

// SignDockerManifest returns a signature for manifest as the specified dockerReference,
// using mech and keyIdentity.
func SignDockerManifest(m []byte, dockerReference string, mech SigningMechanism, keyIdentity string) ([]byte, error) {
	manifestDigest, err := manifest.Digest(m)
	if err != nil {
		return nil, err
	}
	sig := newUntrustedSignature(manifestDigest, dockerReference)
	return sig.sign(mech, keyIdentity)
}

// VerifyDockerManifestSignature checks that unverifiedSignature uses expectedKeyIdentity to sign unverifiedManifest as expectedDockerReference,
// using mech.
func VerifyDockerManifestSignature(unverifiedSignature, unverifiedManifest []byte,
	expectedDockerReference string, mech SigningMechanism, expectedKeyIdentity string) (*Signature, error) {
	expectedRef, err := reference.ParseNormalizedNamed(expectedDockerReference)
	if err != nil {
		return nil, err
	}
	sig, err := verifyAndExtractSignature(mech, unverifiedSignature, signatureAcceptanceRules{
		validateKeyIdentity: func(keyIdentity string) error {
			if keyIdentity != expectedKeyIdentity {
				return InvalidSignatureError{msg: fmt.Sprintf("Signature by %s does not match expected fingerprint %s", keyIdentity, expectedKeyIdentity)}
			}
			return nil
		},
		validateSignedDockerReference: func(signedDockerReference string) error {
			signedRef, err := reference.ParseNormalizedNamed(signedDockerReference)
			if err != nil {
				return InvalidSignatureError{msg: fmt.Sprintf("Invalid docker reference %s in signature", signedDockerReference)}
			}
			if signedRef.String() != expectedRef.String() {
				return InvalidSignatureError{msg: fmt.Sprintf("Docker reference %s does not match %s",
					signedDockerReference, expectedDockerReference)}
			}
			return nil
		},
		validateSignedDockerManifestDigest: func(signedDockerManifestDigest digest.Digest) error {
			matches, err := manifest.MatchesDigest(unverifiedManifest, signedDockerManifestDigest)
			if err != nil {
				return err
			}
			if !matches {
				return InvalidSignatureError{msg: fmt.Sprintf("Signature for docker digest %q does not match", signedDockerManifestDigest)}
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return sig, nil
}
