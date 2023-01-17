package internal

import (
	"encoding/json"
)

// This is the github.com/sigstore/rekor/pkg/generated/models.Hashedrekord.APIVersion for github.com/sigstore/rekor/pkg/generated/models.HashedrekordV001Schema.
// We could alternatively use github.com/sigstore/rekor/pkg/types/hashedrekord.APIVERSION, but that subpackage adds too many dependencies.
const HashedRekordV001APIVersion = "0.0.1"

// UntrustedRekorSET is a parsed content of the sigstore-signature Rekor SET
// (note that this a signature-specific format, not a format directly used by the Rekor API).
// This corresponds to github.com/sigstore/cosign/bundle.RekorBundle, but we impose a stricter decoder.
type UntrustedRekorSET struct {
	UntrustedSignedEntryTimestamp []byte // A signature over some canonical JSON form of UntrustedPayload
	UntrustedPayload              json.RawMessage
}

type UntrustedRekorPayload struct {
	Body           []byte // In cosign, this is an interface{}, but only a string works
	IntegratedTime int64
	LogIndex       int64
	LogID          string
}

// A compile-time check that UntrustedRekorSET implements json.Unmarshaler
var _ json.Unmarshaler = (*UntrustedRekorSET)(nil)

// UnmarshalJSON implements the json.Unmarshaler interface
func (s *UntrustedRekorSET) UnmarshalJSON(data []byte) error {
	err := s.strictUnmarshalJSON(data)
	if err != nil {
		if formatErr, ok := err.(JSONFormatError); ok {
			err = NewInvalidSignatureError(formatErr.Error())
		}
	}
	return err
}

// strictUnmarshalJSON is UnmarshalJSON, except that it may return the internal JSONFormatError error type.
// Splitting it into a separate function allows us to do the JSONFormatError → InvalidSignatureError in a single place, the caller.
func (s *UntrustedRekorSET) strictUnmarshalJSON(data []byte) error {
	return ParanoidUnmarshalJSONObjectExactFields(data, map[string]interface{}{
		"SignedEntryTimestamp": &s.UntrustedSignedEntryTimestamp,
		"Payload":              &s.UntrustedPayload,
	})
}

// A compile-time check that UntrustedRekorSET and *UntrustedRekorSET implements json.Marshaler
var _ json.Marshaler = UntrustedRekorSET{}
var _ json.Marshaler = (*UntrustedRekorSET)(nil)

// MarshalJSON implements the json.Marshaler interface.
func (s UntrustedRekorSET) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"SignedEntryTimestamp": s.UntrustedSignedEntryTimestamp,
		"Payload":              s.UntrustedPayload,
	})
}

// A compile-time check that UntrustedRekorPayload implements json.Unmarshaler
var _ json.Unmarshaler = (*UntrustedRekorPayload)(nil)

// UnmarshalJSON implements the json.Unmarshaler interface
func (p *UntrustedRekorPayload) UnmarshalJSON(data []byte) error {
	err := p.strictUnmarshalJSON(data)
	if err != nil {
		if formatErr, ok := err.(JSONFormatError); ok {
			err = NewInvalidSignatureError(formatErr.Error())
		}
	}
	return err
}

// strictUnmarshalJSON is UnmarshalJSON, except that it may return the internal JSONFormatError error type.
// Splitting it into a separate function allows us to do the JSONFormatError → InvalidSignatureError in a single place, the caller.
func (p *UntrustedRekorPayload) strictUnmarshalJSON(data []byte) error {
	return ParanoidUnmarshalJSONObjectExactFields(data, map[string]interface{}{
		"body":           &p.Body,
		"integratedTime": &p.IntegratedTime,
		"logIndex":       &p.LogIndex,
		"logID":          &p.LogID,
	})
}

// A compile-time check that UntrustedRekorPayload and *UntrustedRekorPayload implements json.Marshaler
var _ json.Marshaler = UntrustedRekorPayload{}
var _ json.Marshaler = (*UntrustedRekorPayload)(nil)

// MarshalJSON implements the json.Marshaler interface.
func (p UntrustedRekorPayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"body":           p.Body,
		"integratedTime": p.IntegratedTime,
		"logIndex":       p.LogIndex,
		"logID":          p.LogID,
	})
}
