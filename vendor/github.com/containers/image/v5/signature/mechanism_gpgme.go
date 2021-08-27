//go:build !containers_image_openpgp
// +build !containers_image_openpgp

package signature

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/mtrmac/gpgme"
)

// A GPG/OpenPGP signing mechanism, implemented using gpgme.
type gpgmeSigningMechanism struct {
	ctx          *gpgme.Context
	ephemeralDir string // If not "", a directory to be removed on Close()
}

// newGPGSigningMechanismInDirectory returns a new GPG/OpenPGP signing mechanism, using optionalDir if not empty.
// The caller must call .Close() on the returned SigningMechanism.
func newGPGSigningMechanismInDirectory(optionalDir string) (SigningMechanism, error) {
	ctx, err := newGPGMEContext(optionalDir)
	if err != nil {
		return nil, err
	}
	return &gpgmeSigningMechanism{
		ctx:          ctx,
		ephemeralDir: "",
	}, nil
}

// newEphemeralGPGSigningMechanism returns a new GPG/OpenPGP signing mechanism which
// recognizes _only_ public keys from the supplied blob, and returns the identities
// of these keys.
// The caller must call .Close() on the returned SigningMechanism.
func newEphemeralGPGSigningMechanism(blob []byte) (SigningMechanism, []string, error) {
	dir, err := ioutil.TempDir("", "containers-ephemeral-gpg-")
	if err != nil {
		return nil, nil, err
	}
	removeDir := true
	defer func() {
		if removeDir {
			os.RemoveAll(dir)
		}
	}()
	ctx, err := newGPGMEContext(dir)
	if err != nil {
		return nil, nil, err
	}
	mech := &gpgmeSigningMechanism{
		ctx:          ctx,
		ephemeralDir: dir,
	}
	keyIdentities, err := mech.importKeysFromBytes(blob)
	if err != nil {
		return nil, nil, err
	}

	removeDir = false
	return mech, keyIdentities, nil
}

// newGPGMEContext returns a new *gpgme.Context, using optionalDir if not empty.
func newGPGMEContext(optionalDir string) (*gpgme.Context, error) {
	ctx, err := gpgme.New()
	if err != nil {
		return nil, err
	}
	if err = ctx.SetProtocol(gpgme.ProtocolOpenPGP); err != nil {
		return nil, err
	}
	if optionalDir != "" {
		err := ctx.SetEngineInfo(gpgme.ProtocolOpenPGP, "", optionalDir)
		if err != nil {
			return nil, err
		}
	}
	ctx.SetArmor(false)
	ctx.SetTextMode(false)
	return ctx, nil
}

func (m *gpgmeSigningMechanism) Close() error {
	if m.ephemeralDir != "" {
		os.RemoveAll(m.ephemeralDir) // Ignore an error, if any
	}
	return nil
}

// importKeysFromBytes imports public keys from the supplied blob and returns their identities.
// The blob is assumed to have an appropriate format (the caller is expected to know which one).
// NOTE: This may modify long-term state (e.g. key storage in a directory underlying the mechanism);
// but we do not make this public, it can only be used through newEphemeralGPGSigningMechanism.
func (m *gpgmeSigningMechanism) importKeysFromBytes(blob []byte) ([]string, error) {
	inputData, err := gpgme.NewDataBytes(blob)
	if err != nil {
		return nil, err
	}
	res, err := m.ctx.Import(inputData)
	if err != nil {
		return nil, err
	}
	keyIdentities := []string{}
	for _, i := range res.Imports {
		if i.Result == nil {
			keyIdentities = append(keyIdentities, i.Fingerprint)
		}
	}
	return keyIdentities, nil
}

// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
func (m *gpgmeSigningMechanism) SupportsSigning() error {
	return nil
}

// Sign creates a (non-detached) signature of input using keyIdentity.
// Fails with a SigningNotSupportedError if the mechanism does not support signing.
func (m *gpgmeSigningMechanism) Sign(input []byte, keyIdentity string) ([]byte, error) {
	key, err := m.ctx.GetKey(keyIdentity, true)
	if err != nil {
		return nil, err
	}
	inputData, err := gpgme.NewDataBytes(input)
	if err != nil {
		return nil, err
	}
	var sigBuffer bytes.Buffer
	sigData, err := gpgme.NewDataWriter(&sigBuffer)
	if err != nil {
		return nil, err
	}
	if err = m.ctx.Sign([]*gpgme.Key{key}, inputData, sigData, gpgme.SigModeNormal); err != nil {
		return nil, err
	}
	return sigBuffer.Bytes(), nil
}

// Verify parses unverifiedSignature and returns the content and the signer's identity
func (m *gpgmeSigningMechanism) Verify(unverifiedSignature []byte) (contents []byte, keyIdentity string, err error) {
	signedBuffer := bytes.Buffer{}
	signedData, err := gpgme.NewDataWriter(&signedBuffer)
	if err != nil {
		return nil, "", err
	}
	unverifiedSignatureData, err := gpgme.NewDataBytes(unverifiedSignature)
	if err != nil {
		return nil, "", err
	}
	_, sigs, err := m.ctx.Verify(unverifiedSignatureData, nil, signedData)
	if err != nil {
		return nil, "", err
	}
	if len(sigs) != 1 {
		return nil, "", InvalidSignatureError{msg: fmt.Sprintf("Unexpected GPG signature count %d", len(sigs))}
	}
	sig := sigs[0]
	// This is sig.Summary == gpgme.SigSumValid except for key trust, which we handle ourselves
	if sig.Status != nil || sig.Validity == gpgme.ValidityNever || sig.ValidityReason != nil || sig.WrongKeyUsage {
		// FIXME: Better error reporting eventually
		return nil, "", InvalidSignatureError{msg: fmt.Sprintf("Invalid GPG signature: %#v", sig)}
	}
	return signedBuffer.Bytes(), sig.Fingerprint, nil
}

// UntrustedSignatureContents returns UNTRUSTED contents of the signature WITHOUT ANY VERIFICATION,
// along with a short identifier of the key used for signing.
// WARNING: The short key identifier (which corresponds to "Key ID" for OpenPGP keys)
// is NOT the same as a "key identity" used in other calls to this interface, and
// the values may have no recognizable relationship if the public key is not available.
func (m *gpgmeSigningMechanism) UntrustedSignatureContents(untrustedSignature []byte) (untrustedContents []byte, shortKeyIdentifier string, err error) {
	return gpgUntrustedSignatureContents(untrustedSignature)
}
