//go:generate stringer -type=FeatureFlag

package features

import (
	"fmt"
	"sync"
)

type FeatureFlag int

const (
	unused FeatureFlag = iota // unused is used for testing
	//   Deprecated features, these can be removed once stripped from production configs
	PrecertificateRevocation
	StripDefaultSchemePort
	NonCFSSLSigner
	StoreIssuerInfo
	StreamlineOrderAndAuthzs
	V1DisableNewValidations

	//   Currently in-use features
	// Check CAA and respect validationmethods parameter.
	CAAValidationMethods
	// Check CAA and respect accounturi parameter.
	CAAAccountURI
	// EnforceMultiVA causes the VA to block on remote VA PerformValidation
	// requests in order to make a valid/invalid decision with the results.
	EnforceMultiVA
	// MultiVAFullResults will cause the main VA to wait for all of the remote VA
	// results, not just the threshold required to make a decision.
	MultiVAFullResults
	// MandatoryPOSTAsGET forbids legacy unauthenticated GET requests for ACME
	// resources.
	MandatoryPOSTAsGET
	// Allow creation of new registrations in ACMEv1.
	AllowV1Registration
	// StoreRevokerInfo enables storage of the revoker and a bool indicating if the row
	// was checked for extant unrevoked certificates in the blockedKeys table.
	StoreRevokerInfo
	// RestrictRSAKeySizes enables restriction of acceptable RSA public key moduli to
	// the common sizes (2048, 3072, and 4096 bits).
	RestrictRSAKeySizes
	// FasterNewOrdersRateLimit enables use of a separate table for counting the
	// new orders rate limit.
	FasterNewOrdersRateLimit
	// ECDSAForAll enables all accounts, regardless of their presence in the CA's
	// ecdsaAllowedAccounts config value, to get issuance from ECDSA issuers.
	ECDSAForAll
	// ServeRenewalInfo exposes the renewalInfo endpoint in the directory and for
	// GET requests. WARNING: This feature is a draft and highly unstable.
	ServeRenewalInfo
	// GetAuthzReadOnly causes the SA to use its read-only database connection
	// (which is generally pointed at a replica rather than the primary db) when
	// querying the authz2 table.
	GetAuthzReadOnly
	// GetAuthzUseIndex causes the SA to use to add a USE INDEX hint when it
	// queries the authz2 table.
	GetAuthzUseIndex
	// Check the failed authorization limit before doing authz reuse.
	CheckFailedAuthorizationsFirst
	// AllowReRevocation causes the RA to allow the revocation reason of an
	// already-revoked certificate to be updated to `keyCompromise` from any
	// other reason if that compromise is demonstrated by making the second
	// revocation request signed by the certificate keypair.
	AllowReRevocation
	// MozRevocationReasons causes the RA to enforce the following upcoming
	// Mozilla policies regarding revocation:
	// - A subscriber can request that their certificate be revoked with reason
	//   keyCompromise, even without demonstrating that compromise at the time.
	//   However, the cert's pubkey will not be added to the blocked keys list.
	// - When an applicant other than the original subscriber requests that a
	//   certificate be revoked (by demonstrating control over all names in it),
	//   the cert will be revoked with reason cessationOfOperation, regardless of
	//   what revocation reason they request.
	// - When anyone requests that a certificate be revoked by signing the request
	//   with the certificate's keypair, the cert will be revoked with reason
	//   keyCompromise, regardless of what revocation reason they request.
	MozRevocationReasons
)

// List of features and their default value, protected by fMu
var features = map[FeatureFlag]bool{
	unused:                         false,
	CAAValidationMethods:           false,
	CAAAccountURI:                  false,
	EnforceMultiVA:                 false,
	MultiVAFullResults:             false,
	MandatoryPOSTAsGET:             false,
	AllowV1Registration:            true,
	V1DisableNewValidations:        false,
	PrecertificateRevocation:       false,
	StripDefaultSchemePort:         false,
	StoreIssuerInfo:                false,
	StoreRevokerInfo:               false,
	RestrictRSAKeySizes:            false,
	FasterNewOrdersRateLimit:       false,
	NonCFSSLSigner:                 false,
	ECDSAForAll:                    false,
	StreamlineOrderAndAuthzs:       false,
	ServeRenewalInfo:               false,
	GetAuthzReadOnly:               false,
	GetAuthzUseIndex:               false,
	CheckFailedAuthorizationsFirst: false,
	AllowReRevocation:              false,
	MozRevocationReasons:           false,
}

var fMu = new(sync.RWMutex)

var initial = map[FeatureFlag]bool{}

var nameToFeature = make(map[string]FeatureFlag, len(features))

func init() {
	for f, v := range features {
		nameToFeature[f.String()] = f
		initial[f] = v
	}
}

// Set accepts a list of features and whether they should
// be enabled or disabled, it will return a error if passed
// a feature name that it doesn't know
func Set(featureSet map[string]bool) error {
	fMu.Lock()
	defer fMu.Unlock()
	for n, v := range featureSet {
		f, present := nameToFeature[n]
		if !present {
			return fmt.Errorf("feature '%s' doesn't exist", n)
		}
		features[f] = v
	}
	return nil
}

// Enabled returns true if the feature is enabled or false
// if it isn't, it will panic if passed a feature that it
// doesn't know.
func Enabled(n FeatureFlag) bool {
	fMu.RLock()
	defer fMu.RUnlock()
	v, present := features[n]
	if !present {
		panic(fmt.Sprintf("feature '%s' doesn't exist", n.String()))
	}
	return v
}

// Reset resets the features to their initial state
func Reset() {
	fMu.Lock()
	defer fMu.Unlock()
	for k, v := range initial {
		features[k] = v
	}
}
