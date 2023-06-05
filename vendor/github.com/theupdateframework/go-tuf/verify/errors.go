package verify

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrMissingKey           = errors.New("tuf: missing key")
	ErrNoSignatures         = errors.New("tuf: data has no signatures")
	ErrInvalid              = errors.New("tuf: signature verification failed")
	ErrWrongMethod          = errors.New("tuf: invalid signature type")
	ErrWrongMetaType        = errors.New("tuf: meta file has wrong type")
	ErrExists               = errors.New("tuf: key already in db")
	ErrInvalidKey           = errors.New("tuf: invalid key")
	ErrInvalidRole          = errors.New("tuf: invalid role")
	ErrInvalidDelegatedRole = errors.New("tuf: invalid delegated role")
	ErrInvalidKeyID         = errors.New("tuf: invalid key id")
	ErrInvalidThreshold     = errors.New("tuf: invalid role threshold")
	ErrMissingTargetFile    = errors.New("tuf: missing previously listed targets metadata file")
)

type ErrRepeatID struct {
	KeyID string
}

func (e ErrRepeatID) Error() string {
	return fmt.Sprintf("tuf: duplicate key id (%s)", e.KeyID)
}

type ErrUnknownRole struct {
	Role string
}

func (e ErrUnknownRole) Error() string {
	return fmt.Sprintf("tuf: unknown role %q", e.Role)
}

type ErrExpired struct {
	Expired time.Time
}

func (e ErrExpired) Error() string {
	return fmt.Sprintf("expired at %s", e.Expired)
}

type ErrLowVersion struct {
	Actual  int64
	Current int64
}

func (e ErrLowVersion) Error() string {
	return fmt.Sprintf("version %d is lower than current version %d", e.Actual, e.Current)
}

type ErrWrongVersion struct {
	Given    int64
	Expected int64
}

func (e ErrWrongVersion) Error() string {
	return fmt.Sprintf("version %d does not match the expected version %d", e.Given, e.Expected)
}

type ErrRoleThreshold struct {
	Expected int
	Actual   int
}

func (e ErrRoleThreshold) Error() string {
	return "tuf: valid signatures did not meet threshold"
}
