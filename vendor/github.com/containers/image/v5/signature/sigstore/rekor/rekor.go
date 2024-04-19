//go:build !containers_image_rekor_stub
// +build !containers_image_rekor_stub

package rekor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/containers/image/v5/signature/internal"
	signerInternal "github.com/containers/image/v5/signature/sigstore/internal"
	"github.com/go-openapi/strfmt"
	rekor "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/generated/client/entries"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sirupsen/logrus"
)

// WithRekor asks the generated signature to be uploaded to the specified Rekor server,
// and to include a log inclusion proof in the signature.
func WithRekor(rekorURL *url.URL) signerInternal.Option {
	return func(s *signerInternal.SigstoreSigner) error {
		logrus.Debugf("Using Rekor server at %s", rekorURL.Redacted())
		client, err := rekor.GetRekorClient(rekorURL.String(),
			rekor.WithLogger(leveledLoggerForLogrus(logrus.StandardLogger())))
		if err != nil {
			return fmt.Errorf("creating Rekor client: %w", err)
		}
		u := uploader{
			client: client,
		}
		s.RekorUploader = u.uploadKeyOrCert
		return nil
	}
}

// uploader wraps a Rekor client, basically so that we can set RekorUploader to a method instead of an one-off closure.
type uploader struct {
	client *client.Rekor
}

// rekorEntryToSET converts a Rekor log entry into a sigstore “signed entry timestamp”.
func rekorEntryToSET(entry *models.LogEntryAnon) (internal.UntrustedRekorSET, error) {
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
func (u *uploader) uploadEntry(ctx context.Context, proposedEntry models.ProposedEntry) (models.LogEntry, error) {
	params := entries.NewCreateLogEntryParamsWithContext(ctx)
	params.SetProposedEntry(proposedEntry)
	logrus.Debugf("Calling Rekor's CreateLogEntry")
	resp, err := u.client.Entries.CreateLogEntry(params)
	if err != nil {
		// In ordinary operation, we should not get duplicate entries, because our payload contains a timestamp,
		// so it is supposed to be unique; and the default key format, ECDSA p256, also contains a nonce.
		// But conflicts can fairly easily happen during debugging and experimentation, so it pays to handle this.
		var conflictErr *entries.CreateLogEntryConflict
		if errors.As(err, &conflictErr) && conflictErr.Location != "" {
			location := conflictErr.Location.String()
			logrus.Debugf("CreateLogEntry reported a conflict, location = %s", location)
			// We might be able to just GET the returned Location, but let’s use the generated API client.
			// OTOH that requires us to hard-code the URI structure…
			uuidDelimiter := strings.LastIndexByte(location, '/')
			if uuidDelimiter != -1 { // Otherwise the URI is unexpected, and fall through to the bottom
				uuid := location[uuidDelimiter+1:]
				logrus.Debugf("Calling Rekor's NewGetLogEntryByUUIDParamsWithContext")
				params2 := entries.NewGetLogEntryByUUIDParamsWithContext(ctx)
				params2.SetEntryUUID(uuid)
				resp2, err := u.client.Entries.GetLogEntryByUUID(params2)
				if err != nil {
					return nil, fmt.Errorf("Error re-loading previously-created log entry with UUID %s: %w", uuid, err)
				}
				return resp2.GetPayload(), nil
			}
		}
		return nil, fmt.Errorf("Error uploading a log entry: %w", err)
	}
	return resp.GetPayload(), nil
}

// stringPtr returns a pointer to the provided string value.
func stringPtr(s string) *string {
	return &s
}

// uploadKeyOrCert integrates this code into sigstore/internal.Signer.
// Given components of the created signature, it returns a SET that should be added to the signature.
func (u *uploader) uploadKeyOrCert(ctx context.Context, keyOrCertBytes []byte, signatureBytes []byte, payloadBytes []byte) ([]byte, error) {
	payloadHash := sha256.Sum256(payloadBytes) // HashedRecord only accepts SHA-256
	proposedEntry := models.Hashedrekord{
		APIVersion: stringPtr(internal.HashedRekordV001APIVersion),
		Spec: models.HashedrekordV001Schema{
			Data: &models.HashedrekordV001SchemaData{
				Hash: &models.HashedrekordV001SchemaDataHash{
					Algorithm: stringPtr(models.HashedrekordV001SchemaDataHashAlgorithmSha256),
					Value:     stringPtr(hex.EncodeToString(payloadHash[:])),
				},
			},
			Signature: &models.HashedrekordV001SchemaSignature{
				Content: strfmt.Base64(signatureBytes),
				PublicKey: &models.HashedrekordV001SchemaSignaturePublicKey{
					Content: strfmt.Base64(keyOrCertBytes),
				},
			},
		},
	}

	uploadedPayload, err := u.uploadEntry(ctx, &proposedEntry)
	if err != nil {
		return nil, err
	}

	if len(uploadedPayload) != 1 {
		return nil, fmt.Errorf("expected 1 Rekor entry, got %d", len(uploadedPayload))
	}
	var storedEntry *models.LogEntryAnon
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
