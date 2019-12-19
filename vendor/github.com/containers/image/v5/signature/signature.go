// Note: Consider the API unstable until the code supports at least three different image formats or transports.

// NOTE: Keep this in sync with docs/atomic-signature.md and docs/atomic-signature-embedded.json!

package signature

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/containers/image/v5/version"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	signatureType = "atomic container signature"
)

// InvalidSignatureError is returned when parsing an invalid signature.
type InvalidSignatureError struct {
	msg string
}

func (err InvalidSignatureError) Error() string {
	return err.msg
}

// Signature is a parsed content of a signature.
// The only way to get this structure from a blob should be as a return value from a successful call to verifyAndExtractSignature below.
type Signature struct {
	DockerManifestDigest digest.Digest
	DockerReference      string // FIXME: more precise type?
}

// untrustedSignature is a parsed content of a signature.
type untrustedSignature struct {
	UntrustedDockerManifestDigest digest.Digest
	UntrustedDockerReference      string // FIXME: more precise type?
	UntrustedCreatorID            *string
	// This is intentionally an int64; the native JSON float64 type would allow to represent _some_ sub-second precision,
	// but not nearly enough (with current timestamp values, a single unit in the last place is on the order of hundreds of nanoseconds).
	// So, this is explicitly an int64, and we reject fractional values. If we did need more precise timestamps eventually,
	// we would add another field, UntrustedTimestampNS int64.
	UntrustedTimestamp *int64
}

// UntrustedSignatureInformation is information available in an untrusted signature.
// This may be useful when debugging signature verification failures,
// or when managing a set of signatures on a single image.
//
// WARNING: Do not use the contents of this for ANY security decisions,
// and be VERY CAREFUL about showing this information to humans in any way which suggest that these values “are probably” reliable.
// There is NO REASON to expect the values to be correct, or not intentionally misleading
// (including things like “✅ Verified by $authority”)
type UntrustedSignatureInformation struct {
	UntrustedDockerManifestDigest digest.Digest
	UntrustedDockerReference      string // FIXME: more precise type?
	UntrustedCreatorID            *string
	UntrustedTimestamp            *time.Time
	UntrustedShortKeyIdentifier   string
}

// newUntrustedSignature returns an untrustedSignature object with
// the specified primary contents and appropriate metadata.
func newUntrustedSignature(dockerManifestDigest digest.Digest, dockerReference string) untrustedSignature {
	// Use intermediate variables for these values so that we can take their addresses.
	// Golang guarantees that they will have a new address on every execution.
	creatorID := "atomic " + version.Version
	timestamp := time.Now().Unix()
	return untrustedSignature{
		UntrustedDockerManifestDigest: dockerManifestDigest,
		UntrustedDockerReference:      dockerReference,
		UntrustedCreatorID:            &creatorID,
		UntrustedTimestamp:            &timestamp,
	}
}

// Compile-time check that untrustedSignature implements json.Marshaler
var _ json.Marshaler = (*untrustedSignature)(nil)

// MarshalJSON implements the json.Marshaler interface.
func (s untrustedSignature) MarshalJSON() ([]byte, error) {
	if s.UntrustedDockerManifestDigest == "" || s.UntrustedDockerReference == "" {
		return nil, errors.New("Unexpected empty signature content")
	}
	critical := map[string]interface{}{
		"type":     signatureType,
		"image":    map[string]string{"docker-manifest-digest": s.UntrustedDockerManifestDigest.String()},
		"identity": map[string]string{"docker-reference": s.UntrustedDockerReference},
	}
	optional := map[string]interface{}{}
	if s.UntrustedCreatorID != nil {
		optional["creator"] = *s.UntrustedCreatorID
	}
	if s.UntrustedTimestamp != nil {
		optional["timestamp"] = *s.UntrustedTimestamp
	}
	signature := map[string]interface{}{
		"critical": critical,
		"optional": optional,
	}
	return json.Marshal(signature)
}

// Compile-time check that untrustedSignature implements json.Unmarshaler
var _ json.Unmarshaler = (*untrustedSignature)(nil)

// UnmarshalJSON implements the json.Unmarshaler interface
func (s *untrustedSignature) UnmarshalJSON(data []byte) error {
	err := s.strictUnmarshalJSON(data)
	if err != nil {
		if formatErr, ok := err.(jsonFormatError); ok {
			err = InvalidSignatureError{msg: formatErr.Error()}
		}
	}
	return err
}

// strictUnmarshalJSON is UnmarshalJSON, except that it may return the internal jsonFormatError error type.
// Splitting it into a separate function allows us to do the jsonFormatError → InvalidSignatureError in a single place, the caller.
func (s *untrustedSignature) strictUnmarshalJSON(data []byte) error {
	var critical, optional json.RawMessage
	if err := paranoidUnmarshalJSONObjectExactFields(data, map[string]interface{}{
		"critical": &critical,
		"optional": &optional,
	}); err != nil {
		return err
	}

	var creatorID string
	var timestamp float64
	var gotCreatorID, gotTimestamp = false, false
	if err := paranoidUnmarshalJSONObject(optional, func(key string) interface{} {
		switch key {
		case "creator":
			gotCreatorID = true
			return &creatorID
		case "timestamp":
			gotTimestamp = true
			return &timestamp
		default:
			var ignore interface{}
			return &ignore
		}
	}); err != nil {
		return err
	}
	if gotCreatorID {
		s.UntrustedCreatorID = &creatorID
	}
	if gotTimestamp {
		intTimestamp := int64(timestamp)
		if float64(intTimestamp) != timestamp {
			return InvalidSignatureError{msg: "Field optional.timestamp is not is not an integer"}
		}
		s.UntrustedTimestamp = &intTimestamp
	}

	var t string
	var image, identity json.RawMessage
	if err := paranoidUnmarshalJSONObjectExactFields(critical, map[string]interface{}{
		"type":     &t,
		"image":    &image,
		"identity": &identity,
	}); err != nil {
		return err
	}
	if t != signatureType {
		return InvalidSignatureError{msg: fmt.Sprintf("Unrecognized signature type %s", t)}
	}

	var digestString string
	if err := paranoidUnmarshalJSONObjectExactFields(image, map[string]interface{}{
		"docker-manifest-digest": &digestString,
	}); err != nil {
		return err
	}
	s.UntrustedDockerManifestDigest = digest.Digest(digestString)

	return paranoidUnmarshalJSONObjectExactFields(identity, map[string]interface{}{
		"docker-reference": &s.UntrustedDockerReference,
	})
}

// Sign formats the signature and returns a blob signed using mech and keyIdentity
// (If it seems surprising that this is a method on untrustedSignature, note that there
// isn’t a good reason to think that a key used by the user is trusted by any component
// of the system just because it is a private key — actually the presence of a private key
// on the system increases the likelihood of an a successful attack on that private key
// on that particular system.)
func (s untrustedSignature) sign(mech SigningMechanism, keyIdentity string) ([]byte, error) {
	json, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	return mech.Sign(json, keyIdentity)
}

// signatureAcceptanceRules specifies how to decide whether an untrusted signature is acceptable.
// We centralize the actual parsing and data extraction in verifyAndExtractSignature; this supplies
// the policy.  We use an object instead of supplying func parameters to verifyAndExtractSignature
// because the functions have the same or similar types, so there is a risk of exchanging the functions;
// named members of this struct are more explicit.
type signatureAcceptanceRules struct {
	validateKeyIdentity                func(string) error
	validateSignedDockerReference      func(string) error
	validateSignedDockerManifestDigest func(digest.Digest) error
}

// verifyAndExtractSignature verifies that unverifiedSignature has been signed, and that its principial components
// match expected values, both as specified by rules, and returns it
func verifyAndExtractSignature(mech SigningMechanism, unverifiedSignature []byte, rules signatureAcceptanceRules) (*Signature, error) {
	signed, keyIdentity, err := mech.Verify(unverifiedSignature)
	if err != nil {
		return nil, err
	}
	if err := rules.validateKeyIdentity(keyIdentity); err != nil {
		return nil, err
	}

	var unmatchedSignature untrustedSignature
	if err := json.Unmarshal(signed, &unmatchedSignature); err != nil {
		return nil, InvalidSignatureError{msg: err.Error()}
	}
	if err := rules.validateSignedDockerManifestDigest(unmatchedSignature.UntrustedDockerManifestDigest); err != nil {
		return nil, err
	}
	if err := rules.validateSignedDockerReference(unmatchedSignature.UntrustedDockerReference); err != nil {
		return nil, err
	}
	// signatureAcceptanceRules have accepted this value.
	return &Signature{
		DockerManifestDigest: unmatchedSignature.UntrustedDockerManifestDigest,
		DockerReference:      unmatchedSignature.UntrustedDockerReference,
	}, nil
}

// GetUntrustedSignatureInformationWithoutVerifying extracts information available in an untrusted signature,
// WITHOUT doing any cryptographic verification.
// This may be useful when debugging signature verification failures,
// or when managing a set of signatures on a single image.
//
// WARNING: Do not use the contents of this for ANY security decisions,
// and be VERY CAREFUL about showing this information to humans in any way which suggest that these values “are probably” reliable.
// There is NO REASON to expect the values to be correct, or not intentionally misleading
// (including things like “✅ Verified by $authority”)
func GetUntrustedSignatureInformationWithoutVerifying(untrustedSignatureBytes []byte) (*UntrustedSignatureInformation, error) {
	// NOTE: This should eventualy do format autodetection.
	mech, _, err := NewEphemeralGPGSigningMechanism([]byte{})
	if err != nil {
		return nil, err
	}
	defer mech.Close()

	untrustedContents, shortKeyIdentifier, err := mech.UntrustedSignatureContents(untrustedSignatureBytes)
	if err != nil {
		return nil, err
	}
	var untrustedDecodedContents untrustedSignature
	if err := json.Unmarshal(untrustedContents, &untrustedDecodedContents); err != nil {
		return nil, InvalidSignatureError{msg: err.Error()}
	}

	var timestamp *time.Time // = nil
	if untrustedDecodedContents.UntrustedTimestamp != nil {
		ts := time.Unix(*untrustedDecodedContents.UntrustedTimestamp, 0)
		timestamp = &ts
	}
	return &UntrustedSignatureInformation{
		UntrustedDockerManifestDigest: untrustedDecodedContents.UntrustedDockerManifestDigest,
		UntrustedDockerReference:      untrustedDecodedContents.UntrustedDockerReference,
		UntrustedCreatorID:            untrustedDecodedContents.UntrustedCreatorID,
		UntrustedTimestamp:            timestamp,
		UntrustedShortKeyIdentifier:   shortKeyIdentifier,
	}, nil
}
