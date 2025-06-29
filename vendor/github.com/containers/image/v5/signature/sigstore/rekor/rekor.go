package rekor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/containers/image/v5/signature/internal"
	signerInternal "github.com/containers/image/v5/signature/sigstore/internal"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
)

const (
	// defaultRetryCount is the default number of retries
	defaultRetryCount = 3
)

// WithRekor asks the generated signature to be uploaded to the specified Rekor server,
// and to include a log inclusion proof in the signature.
func WithRekor(rekorURL *url.URL) signerInternal.Option {
	return func(s *signerInternal.SigstoreSigner) error {
		logrus.Debugf("Using Rekor server at %s", rekorURL.Redacted())
		client := newRekorClient(rekorURL)
		s.RekorUploader = client.uploadKeyOrCert
		return nil
	}
}

// rekorClient allows uploading entries to Rekor.
type rekorClient struct {
	rekorURL   *url.URL // Only Scheme and Host is actually used, consistent with github.com/sigstore/rekor/pkg/client.
	basePath   string
	httpClient *http.Client
}

// newRekorClient creates a rekorClient for rekorURL.
func newRekorClient(rekorURL *url.URL) *rekorClient {
	retryableClient := retryablehttp.NewClient()
	retryableClient.RetryMax = defaultRetryCount
	retryableClient.Logger = leveledLoggerForLogrus(logrus.StandardLogger())
	basePath := rekorURL.Path
	if !strings.HasPrefix(basePath, "/") { // Includes basePath == "", i.e. URL just a https://hostname
		basePath = "/" + basePath
	}
	return &rekorClient{
		rekorURL:   rekorURL,
		basePath:   basePath,
		httpClient: retryableClient.StandardClient(),
	}
}

// rekorEntryToSET converts a Rekor log entry into a sigstore “signed entry timestamp”.
func rekorEntryToSET(entry *rekorLogEntryAnon) (internal.UntrustedRekorSET, error) {
	// We could plausibly call entry.Validate() here; that mostly just uses unnecessary reflection instead of direct == nil checks.
	// Right now the only extra validation .Validate() does is *entry.LogIndex >= 0 and a regex check on *entry.LogID;
	// we don’t particularly care about either of these (notably signature verification only uses the Body value).
	if entry.Verification == nil || entry.IntegratedTime == nil || entry.LogIndex == nil || entry.LogID == nil {
		return internal.UntrustedRekorSET{}, fmt.Errorf("invalid Rekor entry (missing data): %#v", *entry)
	}
	bodyBase64, ok := entry.Body.(string)
	if !ok {
		return internal.UntrustedRekorSET{}, fmt.Errorf("unexpected Rekor entry body type: %#v", entry.Body)
	}
	body, err := base64.StdEncoding.DecodeString(bodyBase64)
	if err != nil {
		return internal.UntrustedRekorSET{}, fmt.Errorf("error parsing Rekor entry body: %w", err)
	}
	payloadJSON, err := internal.UntrustedRekorPayload{
		Body:           body,
		IntegratedTime: *entry.IntegratedTime,
		LogIndex:       *entry.LogIndex,
		LogID:          *entry.LogID,
	}.MarshalJSON()
	if err != nil {
		return internal.UntrustedRekorSET{}, err
	}

	return internal.UntrustedRekorSET{
		UntrustedSignedEntryTimestamp: entry.Verification.SignedEntryTimestamp,
		UntrustedPayload:              payloadJSON,
	}, nil
}

// uploadEntry ensures proposedEntry exists in Rekor (usually uploading it), and returns the resulting log entry.
func (r *rekorClient) uploadEntry(ctx context.Context, proposedEntry rekorProposedEntry) (rekorLogEntry, error) {
	logrus.Debugf("Calling Rekor's CreateLogEntry")
	resp, err := r.createLogEntry(ctx, proposedEntry)
	if err != nil {
		// In ordinary operation, we should not get duplicate entries, because our payload contains a timestamp,
		// so it is supposed to be unique; and the default key format, ECDSA p256, also contains a nonce.
		// But conflicts can fairly easily happen during debugging and experimentation, so it pays to handle this.
		var conflictErr *createLogEntryConflictError
		if errors.As(err, &conflictErr) && conflictErr.location != "" {
			location := conflictErr.location
			logrus.Debugf("CreateLogEntry reported a conflict, location = %s", location)
			// We might be able to just GET the returned Location, but let’s use the formal API method.
			// OTOH that requires us to hard-code the URI structure…
			uuidDelimiter := strings.LastIndexByte(location, '/')
			if uuidDelimiter != -1 { // Otherwise the URI is unexpected, and fall through to the bottom
				uuid := location[uuidDelimiter+1:]
				logrus.Debugf("Calling Rekor's NewGetLogEntryByUUIDParamsWithContext")
				resp2, err := r.getLogEntryByUUID(ctx, uuid)
				if err != nil {
					return nil, fmt.Errorf("Error re-loading previously-created log entry with UUID %s: %w", uuid, err)
				}
				return resp2, nil
			}
		}
		return nil, fmt.Errorf("Error uploading a log entry: %w", err)
	}
	return resp, nil
}

// stringPointer is a helper to create *string fields in JSON data.
func stringPointer(s string) *string {
	return &s
}

// uploadKeyOrCert integrates this code into sigstore/internal.Signer.
// Given components of the created signature, it returns a SET that should be added to the signature.
func (r *rekorClient) uploadKeyOrCert(ctx context.Context, keyOrCertBytes []byte, signatureBytes []byte, payloadBytes []byte) ([]byte, error) {
	payloadHash := sha256.Sum256(payloadBytes) // HashedRecord only accepts SHA-256
	hashedRekordSpec, err := json.Marshal(internal.RekorHashedrekordV001Schema{
		Data: &internal.RekorHashedrekordV001SchemaData{
			Hash: &internal.RekorHashedrekordV001SchemaDataHash{
				Algorithm: stringPointer(internal.RekorHashedrekordV001SchemaDataHashAlgorithmSha256),
				Value:     stringPointer(hex.EncodeToString(payloadHash[:])),
			},
		},
		Signature: &internal.RekorHashedrekordV001SchemaSignature{
			Content: signatureBytes,
			PublicKey: &internal.RekorHashedrekordV001SchemaSignaturePublicKey{
				Content: keyOrCertBytes,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	proposedEntry := internal.RekorHashedrekord{
		APIVersion: stringPointer(internal.RekorHashedRekordV001APIVersion),
		Spec:       hashedRekordSpec,
	}

	uploadedPayload, err := r.uploadEntry(ctx, &proposedEntry)
	if err != nil {
		return nil, err
	}

	if len(uploadedPayload) != 1 {
		return nil, fmt.Errorf("expected 1 Rekor entry, got %d", len(uploadedPayload))
	}
	var storedEntry *rekorLogEntryAnon
	// This “loop” extracts the single value from the uploadedPayload map.
	for _, p := range uploadedPayload {
		storedEntry = &p
		break
	}

	rekorBundle, err := rekorEntryToSET(storedEntry)
	if err != nil {
		return nil, err
	}
	rekorSETBytes, err := json.Marshal(rekorBundle)
	if err != nil {
		return nil, err
	}
	return rekorSETBytes, nil
}
