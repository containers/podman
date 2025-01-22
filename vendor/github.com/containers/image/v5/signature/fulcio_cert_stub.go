//go:build containers_image_fulcio_stub
// +build containers_image_fulcio_stub

package signature

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
)

type fulcioTrustRoot struct {
	caCertificates *x509.CertPool
	oidcIssuer     string
	subjectEmail   string
}

func (f *fulcioTrustRoot) validate() error {
	return errors.New("fulcio disabled at compile-time")
}

func verifyRekorFulcio(rekorPublicKey *ecdsa.PublicKey, fulcioTrustRoot *fulcioTrustRoot, untrustedRekorSET []byte,
	untrustedCertificateBytes []byte, untrustedIntermediateChainBytes []byte, untrustedBase64Signature string,
	untrustedPayloadBytes []byte) (crypto.PublicKey, error) {
	return nil, errors.New("fulcio disabled at compile-time")

}
