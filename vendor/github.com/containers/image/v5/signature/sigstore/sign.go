package sigstore

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature/internal"
	sigstoreSignature "github.com/sigstore/sigstore/pkg/signature"
)

// SignDockerManifestWithPrivateKeyFileUnstable returns a signature for manifest as the specified dockerReference,
// using a private key and an optional passphrase.
//
// Yes, this returns an internal type, and should currently not be used outside of c/image.
// There is NO COMITTMENT TO STABLE API.
func SignDockerManifestWithPrivateKeyFileUnstable(m []byte, dockerReference reference.Named, privateKeyFile string, passphrase []byte) (signature.Sigstore, error) {
	privateKeyPEM, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return signature.Sigstore{}, fmt.Errorf("reading private key from %s: %w", privateKeyFile, err)
	}
	signer, err := loadPrivateKey(privateKeyPEM, passphrase)
	if err != nil {
		return signature.Sigstore{}, fmt.Errorf("initializing private key: %w", err)
	}

	return signDockerManifest(m, dockerReference, signer)
}

func signDockerManifest(m []byte, dockerReference reference.Named, signer sigstoreSignature.Signer) (signature.Sigstore, error) {
	if reference.IsNameOnly(dockerReference) {
		return signature.Sigstore{}, fmt.Errorf("reference %s can’t be signed, it has neither a tag nor a digest", dockerReference.String())
	}
	manifestDigest, err := manifest.Digest(m)
	if err != nil {
		return signature.Sigstore{}, err
	}
	// sigstore/cosign completely ignores dockerReference for actual policy decisions.
	// They record the repo (but NOT THE TAG) in the value; without the tag we can’t detect version rollbacks.
	// So, just do what simple signing does, and cosign won’t mind.
	payloadData := internal.NewUntrustedSigstorePayload(manifestDigest, dockerReference.String())
	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return signature.Sigstore{}, err
	}

	// github.com/sigstore/cosign/internal/pkg/cosign.payloadSigner uses signatureoptions.WithContext(),
	// which seems to be not used by anything. So we don’t bother.
	signatureBytes, err := signer.SignMessage(bytes.NewReader(payloadBytes))
	if err != nil {
		return signature.Sigstore{}, fmt.Errorf("creating signature: %w", err)
	}
	base64Signature := base64.StdEncoding.EncodeToString(signatureBytes)

	return signature.SigstoreFromComponents(signature.SigstoreSignatureMIMEType,
		payloadBytes,
		map[string]string{
			signature.SigstoreSignatureAnnotationKey: base64Signature,
		}), nil
}
