// Policy evaluation for prSigstoreSigned.

package signature

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature/internal"
	digest "github.com/opencontainers/go-digest"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

// loadBytesFromDataOrPath ensures there is at most one of ${prefix}Data and ${prefix}Path set,
// and returns the referenced data, or nil if neither is set.
func loadBytesFromDataOrPath(prefix string, data []byte, path string) ([]byte, error) {
	switch {
	case data != nil && path != "":
		return nil, fmt.Errorf(`Internal inconsistency: both "%sPath" and "%sData" specified`, prefix, prefix)
	case path != "":
		d, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return d, nil
	case data != nil:
		return data, nil
	default: // Nothing
		return nil, nil
	}
}

// prepareTrustRoot creates a fulcioTrustRoot from the input data.
// (This also prevents external implementations of this interface, ensuring that prSigstoreSignedFulcio is the only one.)
func (f *prSigstoreSignedFulcio) prepareTrustRoot() (*fulcioTrustRoot, error) {
	caCertBytes, err := loadBytesFromDataOrPath("fulcioCA", f.CAData, f.CAPath)
	if err != nil {
		return nil, err
	}
	if caCertBytes == nil {
		return nil, errors.New(`Internal inconsistency: Fulcio specified with neither "caPath" nor "caData"`)
	}
	certs := x509.NewCertPool()
	if ok := certs.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, errors.New("error loading Fulcio CA certificates")
	}
	fulcio := fulcioTrustRoot{
		caCertificates: certs,
		oidcIssuer:     f.OIDCIssuer,
		subjectEmail:   f.SubjectEmail,
	}
	if err := fulcio.validate(); err != nil {
		return nil, err
	}
	return &fulcio, nil
}

// sigstoreSignedTrustRoot contains an already parsed version of the prSigstoreSigned policy
type sigstoreSignedTrustRoot struct {
	publicKey      crypto.PublicKey
	fulcio         *fulcioTrustRoot
	rekorPublicKey *ecdsa.PublicKey
}

func (pr *prSigstoreSigned) prepareTrustRoot() (*sigstoreSignedTrustRoot, error) {
	res := sigstoreSignedTrustRoot{}

	publicKeyPEM, err := loadBytesFromDataOrPath("key", pr.KeyData, pr.KeyPath)
	if err != nil {
		return nil, err
	}
	if publicKeyPEM != nil {
		pk, err := cryptoutils.UnmarshalPEMToPublicKey(publicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("parsing public key: %w", err)
		}
		res.publicKey = pk
	}

	if pr.Fulcio != nil {
		f, err := pr.Fulcio.prepareTrustRoot()
		if err != nil {
			return nil, err
		}
		res.fulcio = f
	}

	rekorPublicKeyPEM, err := loadBytesFromDataOrPath("rekorPublicKey", pr.RekorPublicKeyData, pr.RekorPublicKeyPath)
	if err != nil {
		return nil, err
	}
	if rekorPublicKeyPEM != nil {
		pk, err := cryptoutils.UnmarshalPEMToPublicKey(rekorPublicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("parsing Rekor public key: %w", err)
		}
		pkECDSA, ok := pk.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("Rekor public key is not using ECDSA")

		}
		res.rekorPublicKey = pkECDSA
	}

	return &res, nil
}

func (pr *prSigstoreSigned) isSignatureAuthorAccepted(ctx context.Context, image private.UnparsedImage, sig []byte) (signatureAcceptanceResult, *Signature, error) {
	// We don’t know of a single user of this API, and we might return unexpected values in Signature.
	// For now, just punt.
	return sarRejected, nil, errors.New("isSignatureAuthorAccepted is not implemented for sigstore")
}

func (pr *prSigstoreSigned) isSignatureAccepted(ctx context.Context, image private.UnparsedImage, sig signature.Sigstore) (signatureAcceptanceResult, error) {
	// FIXME: move this to per-context initialization
	trustRoot, err := pr.prepareTrustRoot()
	if err != nil {
		return sarRejected, err
	}

	untrustedAnnotations := sig.UntrustedAnnotations()
	untrustedBase64Signature, ok := untrustedAnnotations[signature.SigstoreSignatureAnnotationKey]
	if !ok {
		return sarRejected, fmt.Errorf("missing %s annotation", signature.SigstoreSignatureAnnotationKey)
	}
	untrustedPayload := sig.UntrustedPayload()

	var publicKey crypto.PublicKey
	switch {
	case trustRoot.publicKey != nil && trustRoot.fulcio != nil: // newPRSigstoreSigned rejects such combinations.
		return sarRejected, errors.New("Internal inconsistency: Both a public key and Fulcio CA specified")
	case trustRoot.publicKey == nil && trustRoot.fulcio == nil: // newPRSigstoreSigned rejects such combinations.
		return sarRejected, errors.New("Internal inconsistency: Neither a public key nor a Fulcio CA specified")

	case trustRoot.publicKey != nil:
		if trustRoot.rekorPublicKey != nil {
			untrustedSET, ok := untrustedAnnotations[signature.SigstoreSETAnnotationKey]
			if !ok { // For user convenience; passing an empty []byte to VerifyRekorSet should work.
				return sarRejected, fmt.Errorf("missing %s annotation", signature.SigstoreSETAnnotationKey)
			}
			// We could use publicKeyPEM directly, but let’s re-marshal to avoid inconsistencies.
			// FIXME: We could just generate DER instead of the full PEM text
			recreatedPublicKeyPEM, err := cryptoutils.MarshalPublicKeyToPEM(trustRoot.publicKey)
			if err != nil {
				// Coverage: The key was loaded from a PEM format, so it’s unclear how this could fail.
				// (PEM is not essential, MarshalPublicKeyToPEM can only fail if marshaling to ASN1.DER fails.)
				return sarRejected, fmt.Errorf("re-marshaling public key to PEM: %w", err)

			}
			// We don’t care about the Rekor timestamp, just about log presence.
			if _, err := internal.VerifyRekorSET(trustRoot.rekorPublicKey, []byte(untrustedSET), recreatedPublicKeyPEM, untrustedBase64Signature, untrustedPayload); err != nil {
				return sarRejected, err
			}
		}
		publicKey = trustRoot.publicKey

	case trustRoot.fulcio != nil:
		if trustRoot.rekorPublicKey == nil { // newPRSigstoreSigned rejects such combinations.
			return sarRejected, errors.New("Internal inconsistency: Fulcio CA specified without a Rekor public key")
		}
		untrustedSET, ok := untrustedAnnotations[signature.SigstoreSETAnnotationKey]
		if !ok { // For user convenience; passing an empty []byte to VerifyRekorSet should correctly reject it anyway.
			return sarRejected, fmt.Errorf("missing %s annotation", signature.SigstoreSETAnnotationKey)
		}
		untrustedCert, ok := untrustedAnnotations[signature.SigstoreCertificateAnnotationKey]
		if !ok { // For user convenience; passing an empty []byte to VerifyRekorSet should correctly reject it anyway.
			return sarRejected, fmt.Errorf("missing %s annotation", signature.SigstoreCertificateAnnotationKey)
		}
		var untrustedIntermediateChainBytes []byte
		if untrustedIntermediateChain, ok := untrustedAnnotations[signature.SigstoreIntermediateCertificateChainAnnotationKey]; ok {
			untrustedIntermediateChainBytes = []byte(untrustedIntermediateChain)
		}
		pk, err := verifyRekorFulcio(trustRoot.rekorPublicKey, trustRoot.fulcio,
			[]byte(untrustedSET), []byte(untrustedCert), untrustedIntermediateChainBytes, untrustedBase64Signature, untrustedPayload)
		if err != nil {
			return sarRejected, err
		}
		publicKey = pk
	}

	if publicKey == nil {
		// Coverage: This should never happen, we have already excluded the possibility in the switch above.
		return sarRejected, fmt.Errorf("Internal inconsistency: publicKey not set before verifying sigstore payload")
	}
	signature, err := internal.VerifySigstorePayload(publicKey, untrustedPayload, untrustedBase64Signature, internal.SigstorePayloadAcceptanceRules{
		ValidateSignedDockerReference: func(ref string) error {
			if !pr.SignedIdentity.matchesDockerReference(image, ref) {
				return PolicyRequirementError(fmt.Sprintf("Signature for identity %s is not accepted", ref))
			}
			return nil
		},
		ValidateSignedDockerManifestDigest: func(digest digest.Digest) error {
			m, _, err := image.Manifest(ctx)
			if err != nil {
				return err
			}
			digestMatches, err := manifest.MatchesDigest(m, digest)
			if err != nil {
				return err
			}
			if !digestMatches {
				return PolicyRequirementError(fmt.Sprintf("Signature for digest %s does not match", digest))
			}
			return nil
		},
	})
	if err != nil {
		return sarRejected, err
	}
	if signature == nil { // A paranoid sanity check that VerifySigstorePayload has returned consistent values
		return sarRejected, errors.New("internal error: VerifySigstorePayload succeeded but returned no data") // Coverage: This should never happen.
	}

	return sarAccepted, nil
}

func (pr *prSigstoreSigned) isRunningImageAllowed(ctx context.Context, image private.UnparsedImage) (bool, error) {
	sigs, err := image.UntrustedSignatures(ctx)
	if err != nil {
		return false, err
	}
	var rejections []error
	foundNonSigstoreSignatures := 0
	foundSigstoreNonAttachments := 0
	for _, s := range sigs {
		sigstoreSig, ok := s.(signature.Sigstore)
		if !ok {
			foundNonSigstoreSignatures++
			continue
		}
		if sigstoreSig.UntrustedMIMEType() != signature.SigstoreSignatureMIMEType {
			foundSigstoreNonAttachments++
			continue
		}

		var reason error
		switch res, err := pr.isSignatureAccepted(ctx, image, sigstoreSig); res {
		case sarAccepted:
			// One accepted signature is enough.
			return true, nil
		case sarRejected:
			reason = err
		case sarUnknown:
			// Huh?! This should not happen at all; treat it as any other invalid value.
			fallthrough
		default:
			reason = fmt.Errorf(`Internal error: Unexpected signature verification result "%s"`, string(res))
		}
		rejections = append(rejections, reason)
	}
	var summary error
	switch len(rejections) {
	case 0:
		if foundNonSigstoreSignatures == 0 && foundSigstoreNonAttachments == 0 {
			// A nice message for the most common case.
			summary = PolicyRequirementError("A signature was required, but no signature exists")
		} else {
			summary = PolicyRequirementError(fmt.Sprintf("A signature was required, but no signature exists (%d non-sigstore signatures, %d sigstore non-signature attachments)",
				foundNonSigstoreSignatures, foundSigstoreNonAttachments))
		}
	case 1:
		summary = rejections[0]
	default:
		var msgs []string
		for _, e := range rejections {
			msgs = append(msgs, e.Error())
		}
		summary = PolicyRequirementError(fmt.Sprintf("None of the signatures were accepted, reasons: %s",
			strings.Join(msgs, "; ")))
	}
	return false, summary
}
