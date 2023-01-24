package signature

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/containers/image/v5/signature/internal"
)

// PRSigstoreSignedOption is way to pass values to NewPRSigstoreSigned
type PRSigstoreSignedOption func(*prSigstoreSigned) error

// PRSigstoreSignedWithKeyPath specifies a value for the "keyPath" field when calling NewPRSigstoreSigned.
func PRSigstoreSignedWithKeyPath(keyPath string) PRSigstoreSignedOption {
	return func(pr *prSigstoreSigned) error {
		if pr.KeyPath != "" {
			return errors.New(`"keyPath" already specified`)
		}
		pr.KeyPath = keyPath
		return nil
	}
}

// PRSigstoreSignedWithKeyData specifies a value for the "keyData" field when calling NewPRSigstoreSigned.
func PRSigstoreSignedWithKeyData(keyData []byte) PRSigstoreSignedOption {
	return func(pr *prSigstoreSigned) error {
		if pr.KeyData != nil {
			return errors.New(`"keyData" already specified`)
		}
		pr.KeyData = keyData
		return nil
	}
}

// PRSigstoreSignedWithFulcio specifies a value for the "fulcio" field when calling NewPRSigstoreSigned.
func PRSigstoreSignedWithFulcio(fulcio PRSigstoreSignedFulcio) PRSigstoreSignedOption {
	return func(pr *prSigstoreSigned) error {
		if pr.Fulcio != nil {
			return errors.New(`"fulcio" already specified`)
		}
		pr.Fulcio = fulcio
		return nil
	}
}

// PRSigstoreSignedWithRekorPublicKeyPath specifies a value for the "rekorPublicKeyPath" field when calling NewPRSigstoreSigned.
func PRSigstoreSignedWithRekorPublicKeyPath(rekorPublicKeyPath string) PRSigstoreSignedOption {
	return func(pr *prSigstoreSigned) error {
		if pr.RekorPublicKeyPath != "" {
			return errors.New(`"rekorPublicKeyPath" already specified`)
		}
		pr.RekorPublicKeyPath = rekorPublicKeyPath
		return nil
	}
}

// PRSigstoreSignedWithRekorPublicKeyData specifies a value for the "rekorPublicKeyData" field when calling NewPRSigstoreSigned.
func PRSigstoreSignedWithRekorPublicKeyData(rekorPublicKeyData []byte) PRSigstoreSignedOption {
	return func(pr *prSigstoreSigned) error {
		if pr.RekorPublicKeyData != nil {
			return errors.New(`"rekorPublicKeyData" already specified`)
		}
		pr.RekorPublicKeyData = rekorPublicKeyData
		return nil
	}
}

// PRSigstoreSignedWithSignedIdentity specifies a value for the "signedIdentity" field when calling NewPRSigstoreSigned.
func PRSigstoreSignedWithSignedIdentity(signedIdentity PolicyReferenceMatch) PRSigstoreSignedOption {
	return func(pr *prSigstoreSigned) error {
		if pr.SignedIdentity != nil {
			return errors.New(`"signedIdentity" already specified`)
		}
		pr.SignedIdentity = signedIdentity
		return nil
	}
}

// newPRSigstoreSigned is NewPRSigstoreSigned, except it returns the private type.
func newPRSigstoreSigned(options ...PRSigstoreSignedOption) (*prSigstoreSigned, error) {
	res := prSigstoreSigned{
		prCommon: prCommon{Type: prTypeSigstoreSigned},
	}
	for _, o := range options {
		if err := o(&res); err != nil {
			return nil, err
		}
	}

	keySources := 0
	if res.KeyPath != "" {
		keySources++
	}
	if res.KeyData != nil {
		keySources++
	}
	if res.Fulcio != nil {
		keySources++
	}
	if keySources != 1 {
		return nil, InvalidPolicyFormatError("exactly one of keyPath, keyData and fulcio must be specified")
	}

	if res.RekorPublicKeyPath != "" && res.RekorPublicKeyData != nil {
		return nil, InvalidPolicyFormatError("rekorPublickeyType and rekorPublickeyData cannot be used simultaneously")
	}
	if res.Fulcio != nil && res.RekorPublicKeyPath == "" && res.RekorPublicKeyData == nil {
		return nil, InvalidPolicyFormatError("At least one of RekorPublickeyPath and RekorPublickeyData must be specified if fulcio is used")
	}

	if res.SignedIdentity == nil {
		return nil, InvalidPolicyFormatError("signedIdentity not specified")
	}

	return &res, nil
}

// NewPRSigstoreSigned returns a new "sigstoreSigned" PolicyRequirement based on options.
func NewPRSigstoreSigned(options ...PRSigstoreSignedOption) (PolicyRequirement, error) {
	return newPRSigstoreSigned(options...)
}

// NewPRSigstoreSignedKeyPath returns a new "sigstoreSigned" PolicyRequirement using a KeyPath
func NewPRSigstoreSignedKeyPath(keyPath string, signedIdentity PolicyReferenceMatch) (PolicyRequirement, error) {
	return NewPRSigstoreSigned(
		PRSigstoreSignedWithKeyPath(keyPath),
		PRSigstoreSignedWithSignedIdentity(signedIdentity),
	)
}

// NewPRSigstoreSignedKeyData returns a new "sigstoreSigned" PolicyRequirement using a KeyData
func NewPRSigstoreSignedKeyData(keyData []byte, signedIdentity PolicyReferenceMatch) (PolicyRequirement, error) {
	return NewPRSigstoreSigned(
		PRSigstoreSignedWithKeyData(keyData),
		PRSigstoreSignedWithSignedIdentity(signedIdentity),
	)
}

// Compile-time check that prSigstoreSigned implements json.Unmarshaler.
var _ json.Unmarshaler = (*prSigstoreSigned)(nil)

// UnmarshalJSON implements the json.Unmarshaler interface.
func (pr *prSigstoreSigned) UnmarshalJSON(data []byte) error {
	*pr = prSigstoreSigned{}
	var tmp prSigstoreSigned
	var gotKeyPath, gotKeyData, gotFulcio, gotRekorPublicKeyPath, gotRekorPublicKeyData bool
	var fulcio prSigstoreSignedFulcio
	var signedIdentity json.RawMessage
	if err := internal.ParanoidUnmarshalJSONObject(data, func(key string) interface{} {
		switch key {
		case "type":
			return &tmp.Type
		case "keyPath":
			gotKeyPath = true
			return &tmp.KeyPath
		case "keyData":
			gotKeyData = true
			return &tmp.KeyData
		case "fulcio":
			gotFulcio = true
			return &fulcio
		case "rekorPublicKeyPath":
			gotRekorPublicKeyPath = true
			return &tmp.RekorPublicKeyPath
		case "rekorPublicKeyData":
			gotRekorPublicKeyData = true
			return &tmp.RekorPublicKeyData
		case "signedIdentity":
			return &signedIdentity
		default:
			return nil
		}
	}); err != nil {
		return err
	}

	if tmp.Type != prTypeSigstoreSigned {
		return InvalidPolicyFormatError(fmt.Sprintf("Unexpected policy requirement type \"%s\"", tmp.Type))
	}
	if signedIdentity == nil {
		tmp.SignedIdentity = NewPRMMatchRepoDigestOrExact()
	} else {
		si, err := newPolicyReferenceMatchFromJSON(signedIdentity)
		if err != nil {
			return err
		}
		tmp.SignedIdentity = si
	}

	var opts []PRSigstoreSignedOption
	if gotKeyPath {
		opts = append(opts, PRSigstoreSignedWithKeyPath(tmp.KeyPath))
	}
	if gotKeyData {
		opts = append(opts, PRSigstoreSignedWithKeyData(tmp.KeyData))
	}
	if gotFulcio {
		opts = append(opts, PRSigstoreSignedWithFulcio(&fulcio))
	}
	if gotRekorPublicKeyPath {
		opts = append(opts, PRSigstoreSignedWithRekorPublicKeyPath(tmp.RekorPublicKeyPath))
	}
	if gotRekorPublicKeyData {
		opts = append(opts, PRSigstoreSignedWithRekorPublicKeyData(tmp.RekorPublicKeyData))
	}
	opts = append(opts, PRSigstoreSignedWithSignedIdentity(tmp.SignedIdentity))

	res, err := newPRSigstoreSigned(opts...)
	if err != nil {
		return err
	}
	*pr = *res
	return nil
}

// PRSigstoreSignedFulcioOption is a way to pass values to NewPRSigstoreSignedFulcio
type PRSigstoreSignedFulcioOption func(*prSigstoreSignedFulcio) error

// PRSigstoreSignedFulcioWithCAPath specifies a value for the "caPath" field when calling NewPRSigstoreSignedFulcio
func PRSigstoreSignedFulcioWithCAPath(caPath string) PRSigstoreSignedFulcioOption {
	return func(f *prSigstoreSignedFulcio) error {
		if f.CAPath != "" {
			return errors.New(`"caPath" already specified`)
		}
		f.CAPath = caPath
		return nil
	}
}

// PRSigstoreSignedFulcioWithCAData specifies a value for the "caData" field when calling NewPRSigstoreSignedFulcio
func PRSigstoreSignedFulcioWithCAData(caData []byte) PRSigstoreSignedFulcioOption {
	return func(f *prSigstoreSignedFulcio) error {
		if f.CAData != nil {
			return errors.New(`"caData" already specified`)
		}
		f.CAData = caData
		return nil
	}
}

// PRSigstoreSignedFulcioWithOIDCIssuer specifies a value for the "oidcIssuer" field when calling NewPRSigstoreSignedFulcio
func PRSigstoreSignedFulcioWithOIDCIssuer(oidcIssuer string) PRSigstoreSignedFulcioOption {
	return func(f *prSigstoreSignedFulcio) error {
		if f.OIDCIssuer != "" {
			return errors.New(`"oidcIssuer" already specified`)
		}
		f.OIDCIssuer = oidcIssuer
		return nil
	}
}

// PRSigstoreSignedFulcioWithSubjectEmail specifies a value for the "subjectEmail" field when calling NewPRSigstoreSignedFulcio
func PRSigstoreSignedFulcioWithSubjectEmail(subjectEmail string) PRSigstoreSignedFulcioOption {
	return func(f *prSigstoreSignedFulcio) error {
		if f.SubjectEmail != "" {
			return errors.New(`"subjectEmail" already specified`)
		}
		f.SubjectEmail = subjectEmail
		return nil
	}
}

// newPRSigstoreSignedFulcio is NewPRSigstoreSignedFulcio, except it returns the private type
func newPRSigstoreSignedFulcio(options ...PRSigstoreSignedFulcioOption) (*prSigstoreSignedFulcio, error) {
	res := prSigstoreSignedFulcio{}
	for _, o := range options {
		if err := o(&res); err != nil {
			return nil, err
		}
	}

	if res.CAPath != "" && res.CAData != nil {
		return nil, InvalidPolicyFormatError("caPath and caData cannot be used simultaneously")
	}
	if res.CAPath == "" && res.CAData == nil {
		return nil, InvalidPolicyFormatError("At least one of caPath and caData must be specified")
	}
	if res.OIDCIssuer == "" {
		return nil, InvalidPolicyFormatError("oidcIssuer not specified")
	}
	if res.SubjectEmail == "" {
		return nil, InvalidPolicyFormatError("subjectEmail not specified")
	}

	return &res, nil
}

// NewPRSigstoreSignedFulcio returns a PRSigstoreSignedFulcio based on options.
func NewPRSigstoreSignedFulcio(options ...PRSigstoreSignedFulcioOption) (PRSigstoreSignedFulcio, error) {
	return newPRSigstoreSignedFulcio(options...)
}

// Compile-time check that prSigstoreSignedFulcio implements json.Unmarshaler.
var _ json.Unmarshaler = (*prSigstoreSignedFulcio)(nil)

func (f *prSigstoreSignedFulcio) UnmarshalJSON(data []byte) error {
	*f = prSigstoreSignedFulcio{}
	var tmp prSigstoreSignedFulcio
	var gotCAPath, gotCAData, gotOIDCIssuer, gotSubjectEmail bool // = false...
	if err := internal.ParanoidUnmarshalJSONObject(data, func(key string) interface{} {
		switch key {
		case "caPath":
			gotCAPath = true
			return &tmp.CAPath
		case "caData":
			gotCAData = true
			return &tmp.CAData
		case "oidcIssuer":
			gotOIDCIssuer = true
			return &tmp.OIDCIssuer
		case "subjectEmail":
			gotSubjectEmail = true
			return &tmp.SubjectEmail
		default:
			return nil
		}
	}); err != nil {
		return err
	}

	var opts []PRSigstoreSignedFulcioOption
	if gotCAPath {
		opts = append(opts, PRSigstoreSignedFulcioWithCAPath(tmp.CAPath))
	}
	if gotCAData {
		opts = append(opts, PRSigstoreSignedFulcioWithCAData(tmp.CAData))
	}
	if gotOIDCIssuer {
		opts = append(opts, PRSigstoreSignedFulcioWithOIDCIssuer(tmp.OIDCIssuer))
	}
	if gotSubjectEmail {
		opts = append(opts, PRSigstoreSignedFulcioWithSubjectEmail(tmp.SubjectEmail))
	}

	res, err := newPRSigstoreSignedFulcio(opts...)
	if err != nil {
		return err
	}

	*f = *res
	return nil
}
