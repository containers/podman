// Note: Consider the API unstable until the code supports at least three different image formats or transports.

package signature

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/sigstore/rekor/pkg/generated/models"
	"golang.org/x/crypto/openpgp"
)

// SigningMechanism abstracts a way to sign binary blobs and verify their signatures.
// Each mechanism should eventually be closed by calling Close().
// FIXME: Eventually expand on keyIdentity (namespace them between mechanisms to
// eliminate ambiguities, support CA signatures and perhaps other key properties)
type SigningMechanism interface {
	// Close removes resources associated with the mechanism, if any.
	Close() error
	// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
	SupportsSigning() error
	// Sign creates a (non-detached) signature of input using keyIdentity.
	// Fails with a SigningNotSupportedError if the mechanism does not support signing.
	Sign(input []byte, keyIdentity string) ([]byte, error)
	// Verify parses unverifiedSignature and returns the content and the signer's identity
	Verify(unverifiedSignature []byte) (contents []byte, keyIdentity string, err error)
	// UntrustedSignatureContents returns UNTRUSTED contents of the signature WITHOUT ANY VERIFICATION,
	// along with a short identifier of the key used for signing.
	// WARNING: The short key identifier (which corresponds to "Key ID" for OpenPGP keys)
	// is NOT the same as a "key identity" used in other calls to this interface, and
	// the values may have no recognizable relationship if the public key is not available.
	UntrustedSignatureContents(untrustedSignature []byte) (untrustedContents []byte, shortKeyIdentifier string, err error)
}

// SigstoreSigningMechanism abstracts a way to sign binary blobs and verify their signatures.
// Each mechanism should eventually be closed by calling Close().
// TODO: consolidate with SigningMechanism interface above.
type SigstoreSigningMechanism interface {
	// Close removes resources associated with the mechanism, if any.
	Close() error
	// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
	SupportsSigning() error
	GenerateCertificate() error
	// Sign creates a (non-detached) signature of input using ephemeral keys.
	// Fails with a SigningNotSupportedError if the mechanism does not support signing.
	Sign(payload []byte) (signature []byte, err error)
	Upload(signature, payload []byte) (*models.LogEntryAnon, error)
}

// SigningNotSupportedError is returned when trying to sign using a mechanism which does not support that.
type SigningNotSupportedError string

func (err SigningNotSupportedError) Error() string {
	return string(err)
}

// NewGPGSigningMechanism returns a new GPG/OpenPGP signing mechanism for the user’s default
// GPG configuration ($GNUPGHOME / ~/.gnupg)
// The caller must call .Close() on the returned SigningMechanism.
func NewGPGSigningMechanism() (SigningMechanism, error) {
	return newGPGSigningMechanismInDirectory("")
}

// NewEphemeralGPGSigningMechanism returns a new GPG/OpenPGP signing mechanism which
// recognizes _only_ public keys from the supplied blob, and returns the identities
// of these keys.
// The caller must call .Close() on the returned SigningMechanism.
func NewEphemeralGPGSigningMechanism(blob []byte) (SigningMechanism, []string, error) {
	return newEphemeralGPGSigningMechanism(blob)
}

// NewSigstoreSigningMechanism returns a new sigstore signing mechanism.
// The caller must call .Close() on the returned SigningMechanism.
func NewSigstoreSigningMechanism() (SigstoreSigningMechanism, error) {
	return newSigstoreSigningMechanism()
}

// gpgUntrustedSignatureContents returns UNTRUSTED contents of the signature WITHOUT ANY VERIFICATION,
// along with a short identifier of the key used for signing.
// WARNING: The short key identifier (which corresponds to "Key ID" for OpenPGP keys)
// is NOT the same as a "key identity" used in other calls to this interface, and
// the values may have no recognizable relationship if the public key is not available.
func gpgUntrustedSignatureContents(untrustedSignature []byte) (untrustedContents []byte, shortKeyIdentifier string, err error) {
	// This uses the Golang-native OpenPGP implementation instead of gpgme because we are not doing any cryptography.
	md, err := openpgp.ReadMessage(bytes.NewReader(untrustedSignature), openpgp.EntityList{}, nil, nil)
	if err != nil {
		return nil, "", err
	}
	if !md.IsSigned {
		return nil, "", errors.New("The input is not a signature")
	}
	content, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		// Coverage: An error during reading the body can happen only if
		// 1) the message is encrypted, which is not our case (and we don’t give ReadMessage the key
		// to decrypt the contents anyway), or
		// 2) the message is signed AND we give ReadMessage a corresponding public key, which we don’t.
		return nil, "", err
	}

	// Uppercase the key ID for minimal consistency with the gpgme-returned fingerprints
	// (but note that key ID is a suffix of the fingerprint only for V4 keys, not V3)!
	return content, strings.ToUpper(fmt.Sprintf("%016X", md.SignedByKeyId)), nil
}
