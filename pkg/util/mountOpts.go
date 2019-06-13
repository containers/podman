package util

import (
	"strings"

	"github.com/pkg/errors"
)

var (
	// ErrBadMntOption indicates that an invalid mount option was passed.
	ErrBadMntOption = errors.Errorf("invalid mount option")
	// ErrDupeMntOption indicates that a duplicate mount option was passed.
	ErrDupeMntOption = errors.Errorf("duplicate option passed")
)

// ProcessOptions parses the options for a bind mount and ensures that they are
// sensible and follow convention.
func ProcessOptions(options []string) []string {
	var (
		foundbind, foundrw, foundro bool
		rootProp                    string
	)
	for _, opt := range options {
		switch opt {
		case "bind", "rbind":
			foundbind = true
			break
		}
	}
	if !foundbind {
		options = append(options, "rbind")
	}
	for _, opt := range options {
		switch opt {
		case "rw":
			foundrw = true
		case "ro":
			foundro = true
		case "private", "rprivate", "slave", "rslave", "shared", "rshared":
			rootProp = opt
		}
	}
	if !foundrw && !foundro {
		options = append(options, "rw")
	}
	if rootProp == "" {
		options = append(options, "rprivate")
	}
	return options
}

// ProcessTmpfsOptions parses the options for a tmpfs mountpoint and ensures
// that they are sensible and follow convention.
func ProcessTmpfsOptions(options []string) ([]string, error) {
	var (
		foundWrite, foundSize, foundProp, foundMode bool
	)

	baseOpts := []string{"noexec", "nosuid", "nodev"}
	for _, opt := range options {
		// Some options have parameters - size, mode
		splitOpt := strings.SplitN(opt, "=", 2)
		switch splitOpt[0] {
		case "rw", "ro":
			if foundWrite {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of rw and ro can be used")
			}
			foundWrite = true
			baseOpts = append(baseOpts, opt)
		case "private", "rprivate", "slave", "rslave", "shared", "rshared":
			if foundProp {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one root propagation mode can be used")
			}
			foundProp = true
			baseOpts = append(baseOpts, opt)
		case "size":
			if foundSize {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one tmpfs size can be specified")
			}
			foundSize = true
			baseOpts = append(baseOpts, opt)
		case "mode":
			if foundMode {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one tmpfs mode can be specified")
			}
			foundMode = true
			baseOpts = append(baseOpts, opt)
		case "noexec", "nodev", "nosuid":
			// Do nothing. We always include these even if they are
			// not explicitly requested.
		default:
			return nil, errors.Wrapf(ErrBadMntOption, "unknown tmpfs option %q", opt)
		}
	}

	if !foundWrite {
		baseOpts = append(baseOpts, "rw")
	}
	if !foundSize {
		baseOpts = append(baseOpts, "size=65536k")
	}
	if !foundProp {
		baseOpts = append(baseOpts, "rprivate")
	}

	return baseOpts, nil
}
