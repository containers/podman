package core

import (
	"github.com/letsencrypt/boulder/identifier"
)

// PolicyAuthority defines the public interface for the Boulder PA
// TODO(#5891): Move this interface to a more appropriate location.
type PolicyAuthority interface {
	WillingToIssueWildcards(identifiers []identifier.ACMEIdentifier) error
	ChallengesFor(domain identifier.ACMEIdentifier) ([]Challenge, error)
	ChallengeTypeEnabled(t AcmeChallenge) bool
}
