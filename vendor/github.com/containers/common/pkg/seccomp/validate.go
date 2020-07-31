// +build seccomp

package seccomp

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// ValidateProfile does a basic validation for the provided seccomp profile
// string.
func ValidateProfile(content string) error {
	profile := &Seccomp{}
	if err := json.Unmarshal([]byte(content), &profile); err != nil {
		return errors.Wrap(err, "decoding seccomp profile")
	}

	spec, err := setupSeccomp(profile, nil)
	if err != nil {
		return errors.Wrap(err, "create seccomp spec")
	}

	if _, err := BuildFilter(spec); err != nil {
		return errors.Wrap(err, "build seccomp filter")
	}

	return nil
}
