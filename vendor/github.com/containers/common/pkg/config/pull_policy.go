package config

import (
	"fmt"

	"github.com/pkg/errors"
)

// PullPolicy determines how and which images are being pulled from a container
// registry (i.e., docker transport only).
//
// Supported string values are:
// * "always"  <-> PullPolicyAlways
// * "missing" <-> PullPolicyMissing
// * "newer"  <-> PullPolicyNewer
// * "never"   <-> PullPolicyNever
type PullPolicy int

const (
	// Always pull the image.
	PullPolicyAlways PullPolicy = iota
	// Pull the image only if it could not be found in the local containers
	// storage.
	PullPolicyMissing
	// Never pull the image but use the one from the local containers
	// storage.
	PullPolicyNever
	// Pull if the image on the registry is new than the one in the local
	// containers storage.  An image is considered to be newer when the
	// digests are different.  Comparing the time stamps is prone to
	// errors.
	PullPolicyNewer

	// Ideally this should be the first `ioata` but backwards compatibility
	// prevents us from changing the values.
	PullPolicyUnsupported = -1
)

// String converts a PullPolicy into a string.
//
// Supported string values are:
// * "always"  <-> PullPolicyAlways
// * "missing" <-> PullPolicyMissing
// * "newer"   <-> PullPolicyNewer
// * "never"   <-> PullPolicyNever
func (p PullPolicy) String() string {
	switch p {
	case PullPolicyAlways:
		return "always"
	case PullPolicyMissing:
		return "missing"
	case PullPolicyNewer:
		return "newer"
	case PullPolicyNever:
		return "never"
	}
	return fmt.Sprintf("unrecognized policy %d", p)
}

// Validate returns if the pull policy is not supported.
func (p PullPolicy) Validate() error {
	switch p {
	case PullPolicyAlways, PullPolicyMissing, PullPolicyNewer, PullPolicyNever:
		return nil
	default:
		return errors.Errorf("unsupported pull policy %d", p)
	}
}

// ParsePullPolicy parses the string into a pull policy.
//
// Supported string values are:
// * "always"  <-> PullPolicyAlways
// * "missing" <-> PullPolicyMissing (also "ifnotpresent" and "")
// * "newer"   <-> PullPolicyNewer (also "ifnewer")
// * "never"   <-> PullPolicyNever
func ParsePullPolicy(s string) (PullPolicy, error) {
	switch s {
	case "always":
		return PullPolicyAlways, nil
	case "missing", "ifnotpresent", "":
		return PullPolicyMissing, nil
	case "newer", "ifnewer":
		return PullPolicyNewer, nil
	case "never":
		return PullPolicyNever, nil
	default:
		return PullPolicyUnsupported, errors.Errorf("unsupported pull policy %q", s)
	}
}

// Deprecated: please use `ParsePullPolicy` instead.
func ValidatePullPolicy(s string) (PullPolicy, error) {
	return ParsePullPolicy(s)
}
