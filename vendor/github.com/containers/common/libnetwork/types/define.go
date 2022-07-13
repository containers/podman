package types

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrNoSuchNetwork indicates the requested network does not exist
	ErrNoSuchNetwork = errors.New("network not found")

	// ErrInvalidArg indicates that an invalid argument was passed
	ErrInvalidArg = errors.New("invalid argument")

	// ErrNetworkExists indicates that a network with the given name already
	// exists.
	ErrNetworkExists = errors.New("network already exists")

	// NameRegex is a regular expression to validate names.
	// This must NOT be changed.
	NameRegex = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_.-]*$")
	// RegexError is thrown in presence of an invalid name.
	RegexError = fmt.Errorf("names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*: %w", ErrInvalidArg) // nolint:revive // This lint is new and we do not want to break the API.
)
