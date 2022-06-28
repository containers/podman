// +build containers_image_openpgp

package signature

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containers/storage/pkg/homedir"
	"golang.org/x/crypto/openpgp"
)

// A GPG/OpenPGP signing mechanism, implemented using x/crypto/openpgp.
type openpgpSigningMechanism struct {
	keyring openpgp.EntityList
}

// newGPGSigningMechanismInDirectory returns a new GPG/OpenPGP signing mechanism, using optionalDir if not empty.
// The caller must call .Close() on the returned SigningMechanism.
func newGPGSigningMechanismInDirectory(optionalDir string) (SigningMechanism, error) {
	m := &openpgpSigningMechanism{
		keyring: openpgp.EntityList{},
	}

	gpgHome := optionalDir
	if gpgHome == "" {
		gpgHome = os.Getenv("GNUPGHOME")
		if gpgHome == "" {
			gpgHome = path.Join(homedir.Get(), ".gnupg")
		}
	}

	pubring, err := ioutil.ReadFile(path.Join(gpgHome, "pubring.gpg"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		_, err := m.importKeysFromBytes(pubring)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// newEphemeralGPGSigningMechanism returns a new GPG/OpenPGP signing mechanism which
// recognizes _only_ public keys from the supplied blob, and returns the identities
// of these keys.
// The caller must call .Close() on the returned SigningMechanism.
func newEphemeralGPGSigningMechanism(blob []byte) (SigningMechanism, []string, error) {
	m := &openpgpSigningMechanism{
		keyring: openpgp.EntityList{},
	}
	keyIdentities, err := m.importKeysFromBytes(blob)
	if err != nil {
		return nil, nil, err
	}
	return m, keyIdentities, nil
}

func (m *openpgpSigningMechanism) Close() error {
	return nil
}

// importKeysFromBytes imports public keys from the supplied blob and returns their identities.
// The blob is assumed to have an appropriate format (the caller is expected to know which one).
func (m *openpgpSigningMechanism) importKeysFromBytes(blob []byte) ([]string, error) {
	keyring, err := openpgp.ReadKeyRing(bytes.NewReader(blob))
	if err != nil {
		k, e2 := openpgp.ReadArmoredKeyRing(bytes.NewReader(blob))
		if e2 != nil {
			return nil, err // The original error  -- FIXME: is this better?
		}
		keyring = k
	}

	keyIdentities := []string{}
	for _, entity := range keyring {
		if entity.PrimaryKey == nil {
			// Coverage: This should never happen, openpgp.ReadEntity fails with a
			// openpgp.errors.StructuralError instead of returning an entity with this
			// field set to nil.
			continue
		}
		// Uppercase the fingerprint to be compatible with gpgme
		keyIdentities = append(keyIdentities, strings.ToUpper(fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)))
		m.keyring = append(m.keyring, entity)
	}
	return keyIdentities, nil
}

// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
func (m *openpgpSigningMechanism) SupportsSigning() error {
	return SigningNotSupportedError("signing is not supported in github.com/containers/image built with the containers_image_openpgp build tag")
}

// Sign creates a (non-detached) signature of input using keyIdentity.
// Fails with a SigningNotSupportedError if the mechanism does not support signing.
func (m *openpgpSigningMechanism) Sign(input []byte, keyIdentity string) ([]byte, error) {
	return nil, SigningNotSupportedError("signing is not supported in github.com/containers/image built with the containers_image_openpgp build tag")
}

// Verify parses unverifiedSignature and returns the content and the signer's identity
func (m *openpgpSigningMechanism) Verify(unverifiedSignature []byte) (contents []byte, keyIdentity string, err error) {
	md, err := openpgp.ReadMessage(bytes.NewReader(unverifiedSignature), m.keyring, nil, nil)
	if err != nil {
		return nil, "", err
	}
	if !md.IsSigned {
		return nil, "", errors.New("not signed")
	}
	content, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		// Coverage: md.UnverifiedBody.Read only fails if the body is encrypted
		// (and possibly also signed, but it _must_ be encrypted) and the signing
		// “modification detection code” detects a mismatch. But in that case,
		// we would expect the signature verification to fail as well, and that is checked
		// first.  Besides, we are not supplying any decryption keys, so we really
		// can never reach this “encrypted data MDC mismatch” path.
		return nil, "", err
	}
	if md.SignatureError != nil {
		return nil, "", fmt.Errorf("signature error: %v", md.SignatureError)
	}
	if md.SignedBy == nil {
		return nil, "", InvalidSignatureError{msg: fmt.Sprintf("Invalid GPG signature: %#v", md.Signature)}
	}
	if md.Signature != nil {
		if md.Signature.SigLifetimeSecs != nil {
			expiry := md.Signature.CreationTime.Add(time.Duration(*md.Signature.SigLifetimeSecs) * time.Second)
			if time.Now().After(expiry) {
				return nil, "", InvalidSignatureError{msg: fmt.Sprintf("Signature expired on %s", expiry)}
			}
		}
	} else if md.SignatureV3 == nil {
		// Coverage: If md.SignedBy != nil, the final md.UnverifiedBody.Read() either sets one of md.Signature or md.SignatureV3,
		// or sets md.SignatureError.
		return nil, "", InvalidSignatureError{msg: "Unexpected openpgp.MessageDetails: neither Signature nor SignatureV3 is set"}
	}

	// Uppercase the fingerprint to be compatible with gpgme
	return content, strings.ToUpper(fmt.Sprintf("%x", md.SignedBy.PublicKey.Fingerprint)), nil
}

// UntrustedSignatureContents returns UNTRUSTED contents of the signature WITHOUT ANY VERIFICATION,
// along with a short identifier of the key used for signing.
// WARNING: The short key identifier (which correponds to "Key ID" for OpenPGP keys)
// is NOT the same as a "key identity" used in other calls ot this interface, and
// the values may have no recognizable relationship if the public key is not available.
func (m *openpgpSigningMechanism) UntrustedSignatureContents(untrustedSignature []byte) (untrustedContents []byte, shortKeyIdentifier string, err error) {
	return gpgUntrustedSignatureContents(untrustedSignature)
}
