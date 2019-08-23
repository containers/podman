package util

import (
	"strings"

	"github.com/pkg/errors"
)

var (
	// ErrBadMntOption indicates that an invalid mount option was passed.
	ErrBadMntOption = errors.Errorf("invalid mount option")
	// ErrDupeMntOption indicates that a duplicate mount option was passed.
	ErrDupeMntOption = errors.Errorf("duplicate mount option passed")
)

// DefaultMountOptions sets default mount options for ProcessOptions.
type DefaultMountOptions struct {
	Noexec bool
	Nosuid bool
	Nodev  bool
}

// ProcessOptions parses the options for a bind or tmpfs mount and ensures that
// they are sensible and follow convention. The isTmpfs variable controls
// whether extra, tmpfs-specific options will be allowed.
// The defaults variable controls default mount options that will be set. If it
// is not included, they will be set unconditionally.
func ProcessOptions(options []string, isTmpfs bool, defaults *DefaultMountOptions) ([]string, error) {
	var (
		foundWrite, foundSize, foundProp, foundMode, foundExec, foundSuid, foundDev, foundCopyUp, foundBind, foundZ bool
	)

	for _, opt := range options {
		// Some options have parameters - size, mode
		splitOpt := strings.SplitN(opt, "=", 2)
		switch splitOpt[0] {
		case "exec", "noexec":
			if foundExec {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of 'noexec' and 'exec' can be used")
			}
			foundExec = true
		case "suid", "nosuid":
			if foundSuid {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of 'nosuid' and 'suid' can be used")
			}
			foundSuid = true
		case "nodev", "dev":
			if foundDev {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of 'nodev' and 'dev' can be used")
			}
			foundDev = true
		case "rw", "ro":
			if foundWrite {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of 'rw' and 'ro' can be used")
			}
			foundWrite = true
		case "private", "rprivate", "slave", "rslave", "shared", "rshared":
			if foundProp {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one root propagation mode can be used")
			}
			foundProp = true
		case "size":
			if !isTmpfs {
				return nil, errors.Wrapf(ErrBadMntOption, "the 'size' option is only allowed with tmpfs mounts")
			}
			if foundSize {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one tmpfs size can be specified")
			}
			foundSize = true
		case "mode":
			if !isTmpfs {
				return nil, errors.Wrapf(ErrBadMntOption, "the 'mode' option is only allowed with tmpfs mounts")
			}
			if foundMode {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one tmpfs mode can be specified")
			}
			foundMode = true
		case "tmpcopyup":
			if !isTmpfs {
				return nil, errors.Wrapf(ErrBadMntOption, "the 'tmpcopyup' option is only allowed with tmpfs mounts")
			}
			if foundCopyUp {
				return nil, errors.Wrapf(ErrDupeMntOption, "the 'tmpcopyup' option can only be set once")
			}
			foundCopyUp = true
		case "bind", "rbind":
			if isTmpfs {
				return nil, errors.Wrapf(ErrBadMntOption, "the 'bind' and 'rbind' options are not allowed with tmpfs mounts")
			}
			if foundBind {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of 'rbind' and 'bind' can be used")
			}
			foundBind = true
		case "z", "Z":
			if isTmpfs {
				return nil, errors.Wrapf(ErrBadMntOption, "the 'z' and 'Z' options are not allowed with tmpfs mounts")
			}
			if foundZ {
				return nil, errors.Wrapf(ErrDupeMntOption, "only one of 'z' and 'Z' can be used")
			}
		default:
			return nil, errors.Wrapf(ErrBadMntOption, "unknown mount option %q", opt)
		}
	}

	if !foundWrite {
		options = append(options, "rw")
	}
	if !foundProp {
		options = append(options, "rprivate")
	}
	if !foundExec && (defaults == nil || defaults.Noexec) {
		options = append(options, "noexec")
	}
	if !foundSuid && (defaults == nil || defaults.Nosuid) {
		options = append(options, "nosuid")
	}
	if !foundDev && (defaults == nil || defaults.Nodev) {
		options = append(options, "nodev")
	}
	if isTmpfs && !foundCopyUp {
		options = append(options, "tmpcopyup")
	}
	if !isTmpfs && !foundBind {
		options = append(options, "rbind")
	}

	return options, nil
}
