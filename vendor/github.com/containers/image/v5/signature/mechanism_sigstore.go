package signature

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/google/trillian/merkle/logverifier"
	"github.com/google/trillian/merkle/rfc6962/hasher"
	"github.com/pkg/errors"
	fulcioClient "github.com/sigstore/fulcio/pkg/client"
	"github.com/sigstore/fulcio/pkg/generated/client/operations"
	fulcioModels "github.com/sigstore/fulcio/pkg/generated/models"
	rekorClient "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/generated/client/entries"
	"github.com/sigstore/rekor/pkg/generated/client/pubkey"
	"github.com/sigstore/rekor/pkg/generated/models"
	rekord_v001 "github.com/sigstore/rekor/pkg/types/rekord/v0.0.1"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/oauthflow"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

const (
	defaultRekorURL  = "https://rekor.sigstore.dev"
	defaultFulcioURL = "https://fulcio.sigstore.dev"
	oidcIssuer       = "https://oauth2.sigstore.dev/auth"
	oidcClientID     = "sigstore"
)

func rekorServer() string {
	// TODO: Use another means e.g. flag, policy, etc.
	addr := os.Getenv("REKOR_URL")
	if addr != "" {
		return addr
	}
	return defaultRekorURL
}

func fulcioServer() string {
	// TODO: Use another means e.g. flag, policy, etc.
	addr := os.Getenv("FULCIO_URL")
	if addr != "" {
		return addr
	}
	return defaultFulcioURL
}

type sigstoreSigningMechanism struct {
	ctx   context.Context
	cert  []byte
	chain []byte
	*signature.ECDSASignerVerifier
}

// newSigstoreSigningMechanism returns a new sigstore signing mechanism.
// The caller must call .Close() on the returned SigningMechanism.
func newSigstoreSigningMechanism() (SigstoreSigningMechanism, error) {
	return &sigstoreSigningMechanism{
		ctx: context.Background(),
	}, nil
}

// Close removes resources associated with the mechanism, if any.
func (s *sigstoreSigningMechanism) Close() error {
	return nil
}

// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
func (s *sigstoreSigningMechanism) SupportsSigning() error {
	return nil

}

func (s *sigstoreSigningMechanism) GenerateCertificate() error {
	fulcioServer, err := url.Parse(fulcioServer())
	if err != nil {
		return errors.Wrap(err, "parsing Fulcio URL")
	}

	fClient := fulcioClient.New(fulcioServer)
	if fClient == nil {
		return errors.New("error initializing fulcio client")
	}

	// TODO: use debug log
	fmt.Println("Generating ephemeral keys")
	priv, err := generatePrivateKey()
	if err != nil {
		return errors.Wrap(err, "generating private key")
	}

	signer, err := signature.LoadECDSASignerVerifier(priv, crypto.SHA256)
	if err != nil {
		return err
	}

	fmt.Println("Retrieving signed certificate")
	cert, chain, err := getCertForOauthID(priv, fClient.Operations, oidcIssuer, oidcClientID)
	if err != nil {
		return errors.Wrap(err, "retrieving cert")
	}

	s.ECDSASignerVerifier = signer
	s.cert = cert
	s.chain = chain
	return nil
}

func generatePrivateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func getCertForOauthID(priv *ecdsa.PrivateKey, ops operations.ClientService, oidcIssuer string, oidcClientID string) (certPem, chainPem []byte, err error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	tok, err := oauthflow.OIDConnect(oidcIssuer, oidcClientID, "", oauthflow.DefaultIDTokenGetter)
	if err != nil {
		return nil, nil, err
	}

	// Sign the email address as part of the request
	h := sha256.Sum256([]byte(tok.Subject))
	proof, err := ecdsa.SignASN1(rand.Reader, priv, h[:])
	if err != nil {
		return nil, nil, err
	}

	bearerAuth := httptransport.BearerToken(tok.RawString)

	content := strfmt.Base64(pubBytes)
	signedChallenge := strfmt.Base64(proof)
	params := operations.NewSigningCertParams()
	params.SetCertificateRequest(
		&fulcioModels.CertificateRequest{
			PublicKey: &fulcioModels.CertificateRequestPublicKey{
				Algorithm: fulcioModels.CertificateRequestPublicKeyAlgorithmEcdsa,
				Content:   &content,
			},
			SignedEmailAddress: &signedChallenge,
		},
	)

	resp, err := ops.SigningCert(params, bearerAuth)
	if err != nil {
		return nil, nil, err
	}

	// split the cert and the chain
	certBlock, chainPem := pem.Decode([]byte(resp.Payload))
	certPem = pem.EncodeToMemory(certBlock)
	return certPem, chainPem, nil
}

// Sign creates a signature using the payload input.
// Fails with a SigningNotSupportedError if the mechanism does not support signing.
func (s *sigstoreSigningMechanism) Sign(payload []byte) ([]byte, error) {
	return s.SignMessage(bytes.NewReader(payload), options.WithContext(s.ctx))
}

func (s *sigstoreSigningMechanism) Upload(signature, payload []byte) (*models.LogEntryAnon, error) {
	rekorClient, err := rekorClient.GetRekorClient(rekorServer())
	if err != nil {
		return nil, err
	}
	re := rekorEntry(payload, signature, s.cert)
	returnVal := models.Rekord{
		APIVersion: swag.String(re.APIVersion()),
		Spec:       re.RekordObj,
	}
	return doUpload(rekorClient, &returnVal)
}

func doUpload(rekorClient *client.Rekor, pe models.ProposedEntry) (*models.LogEntryAnon, error) {
	params := entries.NewCreateLogEntryParams()
	params.SetProposedEntry(pe)
	resp, err := rekorClient.Entries.CreateLogEntry(params)
	if err != nil {
		// If the entry already exists, we get a specific error.
		// Here, we display the proof and succeed.
		if existsErr, ok := err.(*entries.CreateLogEntryConflict); ok {

			fmt.Println("Signature already exists. Displaying proof")
			uriSplit := strings.Split(existsErr.Location.String(), "/")
			uuid := uriSplit[len(uriSplit)-1]
			return verifyTLogEntry(rekorClient, uuid)
		}
		return nil, err
	}
	// UUID is at the end of location
	for _, p := range resp.Payload {
		return &p, nil
	}
	return nil, errors.New("bad response from server")
}

func rekorEntry(payload, signature, pubKey []byte) rekord_v001.V001Entry {
	return rekord_v001.V001Entry{
		RekordObj: models.RekordV001Schema{
			Data: &models.RekordV001SchemaData{
				Content: strfmt.Base64(payload),
			},
			Signature: &models.RekordV001SchemaSignature{
				Content: strfmt.Base64(signature),
				Format:  models.RekordV001SchemaSignatureFormatX509,
				PublicKey: &models.RekordV001SchemaSignaturePublicKey{
					Content: strfmt.Base64(pubKey),
				},
			},
		},
	}
}

func verifyTLogEntry(rekorClient *client.Rekor, uuid string) (*models.LogEntryAnon, error) {
	params := entries.NewGetLogEntryByUUIDParams()
	params.EntryUUID = uuid

	lep, err := rekorClient.Entries.GetLogEntryByUUID(params)
	if err != nil {
		return nil, err
	}

	if len(lep.Payload) != 1 {
		return nil, errors.New("UUID value can not be extracted")
	}
	e := lep.Payload[params.EntryUUID]

	if e.Verification == nil || e.Verification.InclusionProof == nil {
		return nil, fmt.Errorf("inclusion proof not provided")
	}

	hashes := [][]byte{}
	for _, h := range e.Verification.InclusionProof.Hashes {
		hb, _ := hex.DecodeString(h)
		hashes = append(hashes, hb)
	}

	rootHash, _ := hex.DecodeString(*e.Verification.InclusionProof.RootHash)
	leafHash, _ := hex.DecodeString(params.EntryUUID)

	v := logverifier.New(hasher.DefaultHasher)
	if err := v.VerifyInclusionProof(*e.Verification.InclusionProof.LogIndex, *e.Verification.InclusionProof.TreeSize, hashes, rootHash, leafHash); err != nil {
		return nil, errors.Wrap(err, "verifying inclusion proof")
	}

	// Verify rekor's signature over the SET.
	resp, err := rekorClient.Pubkey.GetPublicKey(pubkey.NewGetPublicKeyParams())
	if err != nil {
		return nil, errors.Wrap(err, "rekor public key")
	}
	rekorPubKey, err := PemToECDSAKey([]byte(resp.Payload))
	if err != nil {
		return nil, errors.Wrap(err, "rekor public key pem to ecdsa")
	}

	payload := BundlePayload{
		Body:           e.Body,
		IntegratedTime: *e.IntegratedTime,
		LogIndex:       *e.LogIndex,
		LogID:          *e.LogID,
	}
	if err := VerifySET(payload, []byte(e.Verification.SignedEntryTimestamp), rekorPubKey); err != nil {
		return nil, errors.Wrap(err, "verifying signedEntryTimestamp")
	}

	return &e, nil
}

func PemToECDSAKey(pemBytes []byte) (*ecdsa.PublicKey, error) {
	pub, err := cryptoutils.UnmarshalPEMToPublicKey(pemBytes)
	if err != nil {
		return nil, err
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("invalid public key: was %T, require *ecdsa.PublicKey", pub)
	}
	return ecdsaPub, nil
}

type BundlePayload struct {
	Body           interface{} `json:"body"`
	IntegratedTime int64       `json:"integratedTime"`
	LogIndex       int64       `json:"logIndex"`
	LogID          string      `json:"logID"`
}

func VerifySET(bundlePayload BundlePayload, signature []byte, pub *ecdsa.PublicKey) error {
	contents, err := json.Marshal(bundlePayload)
	if err != nil {
		return errors.Wrap(err, "marshaling")
	}
	canonicalized, err := jsoncanonicalizer.Transform(contents)
	if err != nil {
		return errors.Wrap(err, "canonicalizing")
	}

	// verify the SET against the public key
	hash := sha256.Sum256(canonicalized)
	if !ecdsa.VerifyASN1(pub, hash[:], signature) {
		return fmt.Errorf("unable to verify")
	}
	return nil
}
