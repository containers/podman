package core

import (
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ocsp"
	"gopkg.in/go-jose/go-jose.v2"

	"github.com/letsencrypt/boulder/identifier"
	"github.com/letsencrypt/boulder/probs"
	"github.com/letsencrypt/boulder/revocation"
)

// AcmeStatus defines the state of a given authorization
type AcmeStatus string

// These statuses are the states of authorizations, challenges, and registrations
const (
	StatusUnknown     = AcmeStatus("unknown")     // Unknown status; the default
	StatusPending     = AcmeStatus("pending")     // In process; client has next action
	StatusProcessing  = AcmeStatus("processing")  // In process; server has next action
	StatusReady       = AcmeStatus("ready")       // Order is ready for finalization
	StatusValid       = AcmeStatus("valid")       // Object is valid
	StatusInvalid     = AcmeStatus("invalid")     // Validation failed
	StatusRevoked     = AcmeStatus("revoked")     // Object no longer valid
	StatusDeactivated = AcmeStatus("deactivated") // Object has been deactivated
)

// AcmeResource values identify different types of ACME resources
type AcmeResource string

// The types of ACME resources
const (
	ResourceNewReg       = AcmeResource("new-reg")
	ResourceNewAuthz     = AcmeResource("new-authz")
	ResourceNewCert      = AcmeResource("new-cert")
	ResourceRevokeCert   = AcmeResource("revoke-cert")
	ResourceRegistration = AcmeResource("reg")
	ResourceChallenge    = AcmeResource("challenge")
	ResourceAuthz        = AcmeResource("authz")
	ResourceKeyChange    = AcmeResource("key-change")
)

// AcmeChallenge values identify different types of ACME challenges
type AcmeChallenge string

// These types are the available challenges
const (
	ChallengeTypeHTTP01    = AcmeChallenge("http-01")
	ChallengeTypeDNS01     = AcmeChallenge("dns-01")
	ChallengeTypeTLSALPN01 = AcmeChallenge("tls-alpn-01")
)

// IsValid tests whether the challenge is a known challenge
func (c AcmeChallenge) IsValid() bool {
	switch c {
	case ChallengeTypeHTTP01, ChallengeTypeDNS01, ChallengeTypeTLSALPN01:
		return true
	default:
		return false
	}
}

// OCSPStatus defines the state of OCSP for a domain
type OCSPStatus string

// These status are the states of OCSP
const (
	OCSPStatusGood    = OCSPStatus("good")
	OCSPStatusRevoked = OCSPStatus("revoked")
)

var OCSPStatusToInt = map[OCSPStatus]int{
	OCSPStatusGood:    ocsp.Good,
	OCSPStatusRevoked: ocsp.Revoked,
}

// DNSPrefix is attached to DNS names in DNS challenges
const DNSPrefix = "_acme-challenge"

type RawCertificateRequest struct {
	CSR JSONBuffer `json:"csr"` // The encoded CSR
}

// Registration objects represent non-public metadata attached
// to account keys.
type Registration struct {
	// Unique identifier
	ID int64 `json:"id,omitempty" db:"id"`

	// Account key to which the details are attached
	Key *jose.JSONWebKey `json:"key"`

	// Contact URIs
	Contact *[]string `json:"contact,omitempty"`

	// Agreement with terms of service
	Agreement string `json:"agreement,omitempty"`

	// InitialIP is the IP address from which the registration was created
	InitialIP net.IP `json:"initialIp"`

	// CreatedAt is the time the registration was created.
	CreatedAt *time.Time `json:"createdAt,omitempty"`

	Status AcmeStatus `json:"status"`
}

// ValidationRecord represents a validation attempt against a specific URL/hostname
// and the IP addresses that were resolved and used
type ValidationRecord struct {
	// SimpleHTTP only
	URL string `json:"url,omitempty"`

	// Shared
	Hostname          string   `json:"hostname"`
	Port              string   `json:"port,omitempty"`
	AddressesResolved []net.IP `json:"addressesResolved,omitempty"`
	AddressUsed       net.IP   `json:"addressUsed,omitempty"`
	// AddressesTried contains a list of addresses tried before the `AddressUsed`.
	// Presently this will only ever be one IP from `AddressesResolved` since the
	// only retry is in the case of a v6 failure with one v4 fallback. E.g. if
	// a record with `AddressesResolved: { 127.0.0.1, ::1 }` were processed for
	// a challenge validation with the IPv6 first flag on and the ::1 address
	// failed but the 127.0.0.1 retry succeeded then the record would end up
	// being:
	// {
	//   ...
	//   AddressesResolved: [ 127.0.0.1, ::1 ],
	//   AddressUsed: 127.0.0.1
	//   AddressesTried: [ ::1 ],
	//   ...
	// }
	AddressesTried []net.IP `json:"addressesTried,omitempty"`
}

func looksLikeKeyAuthorization(str string) error {
	parts := strings.Split(str, ".")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid key authorization: does not look like a key authorization")
	} else if !LooksLikeAToken(parts[0]) {
		return fmt.Errorf("Invalid key authorization: malformed token")
	} else if !LooksLikeAToken(parts[1]) {
		// Thumbprints have the same syntax as tokens in boulder
		// Both are base64-encoded and 32 octets
		return fmt.Errorf("Invalid key authorization: malformed key thumbprint")
	}
	return nil
}

// Challenge is an aggregate of all data needed for any challenges.
//
// Rather than define individual types for different types of
// challenge, we just throw all the elements into one bucket,
// together with the common metadata elements.
type Challenge struct {
	// The type of challenge
	Type AcmeChallenge `json:"type"`

	// The status of this challenge
	Status AcmeStatus `json:"status,omitempty"`

	// Contains the error that occurred during challenge validation, if any
	Error *probs.ProblemDetails `json:"error,omitempty"`

	// A URI to which a response can be POSTed
	URI string `json:"uri,omitempty"`

	// For the V2 API the "URI" field is deprecated in favour of URL.
	URL string `json:"url,omitempty"`

	// Used by http-01, tls-sni-01, tls-alpn-01 and dns-01 challenges
	Token string `json:"token,omitempty"`

	// The expected KeyAuthorization for validation of the challenge. Populated by
	// the RA prior to passing the challenge to the VA. For legacy reasons this
	// field is called "ProvidedKeyAuthorization" because it was initially set by
	// the content of the challenge update POST from the client. It is no longer
	// set that way and should be renamed to "KeyAuthorization".
	// TODO(@cpu): Rename `ProvidedKeyAuthorization` to `KeyAuthorization`.
	ProvidedKeyAuthorization string `json:"keyAuthorization,omitempty"`

	// Contains information about URLs used or redirected to and IPs resolved and
	// used
	ValidationRecord []ValidationRecord `json:"validationRecord,omitempty"`
	// The time at which the server validated the challenge. Required by
	// RFC8555 if status is valid.
	Validated *time.Time `json:"validated,omitempty"`
}

// ExpectedKeyAuthorization computes the expected KeyAuthorization value for
// the challenge.
func (ch Challenge) ExpectedKeyAuthorization(key *jose.JSONWebKey) (string, error) {
	if key == nil {
		return "", fmt.Errorf("Cannot authorize a nil key")
	}

	thumbprint, err := key.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}

	return ch.Token + "." + base64.RawURLEncoding.EncodeToString(thumbprint), nil
}

// RecordsSane checks the sanity of a ValidationRecord object before sending it
// back to the RA to be stored.
func (ch Challenge) RecordsSane() bool {
	if ch.ValidationRecord == nil || len(ch.ValidationRecord) == 0 {
		return false
	}

	switch ch.Type {
	case ChallengeTypeHTTP01:
		for _, rec := range ch.ValidationRecord {
			if rec.URL == "" || rec.Hostname == "" || rec.Port == "" || rec.AddressUsed == nil ||
				len(rec.AddressesResolved) == 0 {
				return false
			}
		}
	case ChallengeTypeTLSALPN01:
		if len(ch.ValidationRecord) > 1 {
			return false
		}
		if ch.ValidationRecord[0].URL != "" {
			return false
		}
		if ch.ValidationRecord[0].Hostname == "" || ch.ValidationRecord[0].Port == "" ||
			ch.ValidationRecord[0].AddressUsed == nil || len(ch.ValidationRecord[0].AddressesResolved) == 0 {
			return false
		}
	case ChallengeTypeDNS01:
		if len(ch.ValidationRecord) > 1 {
			return false
		}
		if ch.ValidationRecord[0].Hostname == "" {
			return false
		}
		return true
	default: // Unsupported challenge type
		return false
	}

	return true
}

// CheckConsistencyForClientOffer checks the fields of a challenge object before it is
// given to the client.
func (ch Challenge) CheckConsistencyForClientOffer() error {
	err := ch.checkConsistency()
	if err != nil {
		return err
	}

	// Before completion, the key authorization field should be empty
	if ch.ProvidedKeyAuthorization != "" {
		return fmt.Errorf("A response to this challenge was already submitted.")
	}
	return nil
}

// CheckConsistencyForValidation checks the fields of a challenge object before it is
// given to the VA.
func (ch Challenge) CheckConsistencyForValidation() error {
	err := ch.checkConsistency()
	if err != nil {
		return err
	}

	// If the challenge is completed, then there should be a key authorization
	return looksLikeKeyAuthorization(ch.ProvidedKeyAuthorization)
}

// checkConsistency checks the sanity of a challenge object before issued to the client.
func (ch Challenge) checkConsistency() error {
	if ch.Status != StatusPending {
		return fmt.Errorf("The challenge is not pending.")
	}

	// There always needs to be a token
	if !LooksLikeAToken(ch.Token) {
		return fmt.Errorf("The token is missing.")
	}
	return nil
}

// StringID is used to generate a ID for challenges associated with new style authorizations.
// This is necessary as these challenges no longer have a unique non-sequential identifier
// in the new storage scheme. This identifier is generated by constructing a fnv hash over the
// challenge token and type and encoding the first 4 bytes of it using the base64 URL encoding.
func (ch Challenge) StringID() string {
	h := fnv.New128a()
	h.Write([]byte(ch.Token))
	h.Write([]byte(ch.Type))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil)[0:4])
}

// Authorization represents the authorization of an account key holder
// to act on behalf of a domain.  This struct is intended to be used both
// internally and for JSON marshaling on the wire.  Any fields that should be
// suppressed on the wire (e.g., ID, regID) must be made empty before marshaling.
type Authorization struct {
	// An identifier for this authorization, unique across
	// authorizations and certificates within this instance.
	ID string `json:"id,omitempty" db:"id"`

	// The identifier for which authorization is being given
	Identifier identifier.ACMEIdentifier `json:"identifier,omitempty" db:"identifier"`

	// The registration ID associated with the authorization
	RegistrationID int64 `json:"regId,omitempty" db:"registrationID"`

	// The status of the validation of this authorization
	Status AcmeStatus `json:"status,omitempty" db:"status"`

	// The date after which this authorization will be no
	// longer be considered valid. Note: a certificate may be issued even on the
	// last day of an authorization's lifetime. The last day for which someone can
	// hold a valid certificate based on an authorization is authorization
	// lifetime + certificate lifetime.
	Expires *time.Time `json:"expires,omitempty" db:"expires"`

	// An array of challenges objects used to validate the
	// applicant's control of the identifier.  For authorizations
	// in process, these are challenges to be fulfilled; for
	// final authorizations, they describe the evidence that
	// the server used in support of granting the authorization.
	//
	// There should only ever be one challenge of each type in this
	// slice and the order of these challenges may not be predictable.
	Challenges []Challenge `json:"challenges,omitempty" db:"-"`

	// Wildcard is a Boulder-specific Authorization field that indicates the
	// authorization was created as a result of an order containing a name with
	// a `*.`wildcard prefix. This will help convey to users that an
	// Authorization with the identifier `example.com` and one DNS-01 challenge
	// corresponds to a name `*.example.com` from an associated order.
	Wildcard bool `json:"wildcard,omitempty" db:"-"`
}

// FindChallengeByStringID will look for a challenge matching the given ID inside
// this authorization. If found, it will return the index of that challenge within
// the Authorization's Challenges array. Otherwise it will return -1.
func (authz *Authorization) FindChallengeByStringID(id string) int {
	for i, c := range authz.Challenges {
		if c.StringID() == id {
			return i
		}
	}
	return -1
}

// SolvedBy will look through the Authorizations challenges, returning the type
// of the *first* challenge it finds with Status: valid, or an error if no
// challenge is valid.
func (authz *Authorization) SolvedBy() (AcmeChallenge, error) {
	if len(authz.Challenges) == 0 {
		return "", fmt.Errorf("Authorization has no challenges")
	}
	for _, chal := range authz.Challenges {
		if chal.Status == StatusValid {
			return chal.Type, nil
		}
	}
	return "", fmt.Errorf("Authorization not solved by any challenge")
}

// JSONBuffer fields get encoded and decoded JOSE-style, in base64url encoding
// with stripped padding.
type JSONBuffer []byte

// MarshalJSON encodes a JSONBuffer for transmission.
func (jb JSONBuffer) MarshalJSON() (result []byte, err error) {
	return json.Marshal(base64.RawURLEncoding.EncodeToString(jb))
}

// UnmarshalJSON decodes a JSONBuffer to an object.
func (jb *JSONBuffer) UnmarshalJSON(data []byte) (err error) {
	var str string
	err = json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	*jb, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(str, "="))
	return
}

// Certificate objects are entirely internal to the server.  The only
// thing exposed on the wire is the certificate itself.
type Certificate struct {
	ID             int64 `db:"id"`
	RegistrationID int64 `db:"registrationID"`

	Serial  string    `db:"serial"`
	Digest  string    `db:"digest"`
	DER     []byte    `db:"der"`
	Issued  time.Time `db:"issued"`
	Expires time.Time `db:"expires"`
}

// CertificateStatus structs are internal to the server. They represent the
// latest data about the status of the certificate, required for OCSP updating
// and for validating that the subscriber has accepted the certificate.
type CertificateStatus struct {
	ID int64 `db:"id"`

	Serial string `db:"serial"`

	// status: 'good' or 'revoked'. Note that good, expired certificates remain
	//   with status 'good' but don't necessarily get fresh OCSP responses.
	Status OCSPStatus `db:"status"`

	// ocspLastUpdated: The date and time of the last time we generated an OCSP
	//   response. If we have never generated one, this has the zero value of
	//   time.Time, i.e. Jan 1 1970.
	OCSPLastUpdated time.Time `db:"ocspLastUpdated"`

	// revokedDate: If status is 'revoked', this is the date and time it was
	//   revoked. Otherwise it has the zero value of time.Time, i.e. Jan 1 1970.
	RevokedDate time.Time `db:"revokedDate"`

	// revokedReason: If status is 'revoked', this is the reason code for the
	//   revocation. Otherwise it is zero (which happens to be the reason
	//   code for 'unspecified').
	RevokedReason revocation.Reason `db:"revokedReason"`

	LastExpirationNagSent time.Time `db:"lastExpirationNagSent"`

	// The encoded and signed OCSP response.
	OCSPResponse []byte `db:"ocspResponse"`

	// For performance reasons[0] we duplicate the `Expires` field of the
	// `Certificates` object/table in `CertificateStatus` to avoid a costly `JOIN`
	// later on just to retrieve this `Time` value. This helps both the OCSP
	// updater and the expiration-mailer stay performant.
	//
	// Similarly, we add an explicit `IsExpired` boolean to `CertificateStatus`
	// table that the OCSP updater so that the database can create a meaningful
	// index on `(isExpired, ocspLastUpdated)` without a `JOIN` on `certificates`.
	// For more detail see Boulder #1864[0].
	//
	// [0]: https://github.com/letsencrypt/boulder/issues/1864
	NotAfter  time.Time `db:"notAfter"`
	IsExpired bool      `db:"isExpired"`

	// TODO(#5152): Change this to an issuance.Issuer(Name)ID after it no longer
	// has to support both IssuerNameIDs and IssuerIDs.
	IssuerID int64
}

// FQDNSet contains the SHA256 hash of the lowercased, comma joined dNSNames
// contained in a certificate.
type FQDNSet struct {
	ID      int64
	SetHash []byte
	Serial  string
	Issued  time.Time
	Expires time.Time
}

// SCTDERs is a convenience type
type SCTDERs [][]byte

// CertDER is a convenience type that helps differentiate what the
// underlying byte slice contains
type CertDER []byte

// SuggestedWindow is a type exposed inside the RenewalInfo resource.
type SuggestedWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// RenewalInfo is a type which is exposed to clients which query the renewalInfo
// endpoint specified in draft-aaron-ari.
type RenewalInfo struct {
	SuggestedWindow SuggestedWindow `json:"suggestedWindow"`
}

// RenewalInfoSimple constructs a `RenewalInfo` object and suggested window
// using a very simple renewal calculation: calculate a point 2/3rds of the way
// through the validity period, then give a 2-day window around that. Both the
// `issued` and `expires` timestamps are expected to be UTC.
func RenewalInfoSimple(issued time.Time, expires time.Time) RenewalInfo {
	validity := expires.Add(time.Second).Sub(issued)
	renewalOffset := validity / time.Duration(3)
	idealRenewal := expires.Add(-renewalOffset)
	return RenewalInfo{
		SuggestedWindow: SuggestedWindow{
			Start: idealRenewal.Add(-24 * time.Hour),
			End:   idealRenewal.Add(24 * time.Hour),
		},
	}
}

// RenewalInfoImmediate constructs a `RenewalInfo` object with a suggested
// window in the past. Per the draft-ietf-acme-ari-00 spec, clients should
// attempt to renew immediately if the suggested window is in the past. The
// passed `now` is assumed to be a timestamp representing the current moment in
// time.
func RenewalInfoImmediate(now time.Time) RenewalInfo {
	oneHourAgo := now.Add(-1 * time.Hour)
	return RenewalInfo{
		SuggestedWindow: SuggestedWindow{
			Start: oneHourAgo,
			End:   oneHourAgo.Add(time.Minute * 30),
		},
	}
}
