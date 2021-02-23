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

type defaultMountOptions struct {
	noexec bool
	nosuid bool
	nodev  bool
}

// ProcessOptions parses the options for a bind or tmpfs mount and ensures that
// they are sensible and follow convention. The isTmpfs variable controls
// whether extra, tmpfs-specific options will be allowed.
// The sourcePath variable, if not empty, contains a bind mount source.
func ProcessOptions(options []string, isTmpfs bool, sourcePath string) ([]string, error) {
	var (
		foundWrite, foundSize, foundProp, foundMode, foundExec, foundSuid, foundDev, foundCopyUp, foundBind, foundZ, foundU bool
	)

	newOptions := make([]string, 0, len(options))
	for _, opt := range options {
		// Some options have parameters - size, mode
		splitOpt := strings.SplitN(opt, "=", 2)
		switch splitOpt[0] {
		case "O":
			if len(options) > 1 {
				return nil, errors.Wrapf(ErrDupeMntOption, "'O' option can not be used with other options")
			}
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
		case "private", "rprivate", "slave", "rslave", "shared", "rshared", "unbindable", "runbindable":
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
				return nil, errors.Wrapf(ErrDupeMntOption, "the 'tmpcopyup' or 'notmpcopyup' option can only be set once")
			}
			foundCopyUp = true
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		case "notmpcopyup":
			if !isTmpfs {
				return nil, errors.Wrapf(ErrBadMntOption, "the 'notmpcopyup' option is only allowed with tmpfs mounts")
			}
			if foundCopyUp {
				return nil, errors.Wrapf(ErrDupeMntOption, "the 'tmpcopyup' or 'notmpcopyup' option can only be set once")
			}
			foundCopyUp = true
			// do not propagate notmpcopyup to the OCI runtime
			continue
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
			foundZ = true
		case "U":
			if foundU {
				return nil, errors.Wrapf(ErrDupeMntOption, "the 'U' option can only be set once")
			}
			foundU = true
		default:
			return nil, errors.Wrapf(ErrBadMntOption, "unknown mount option %q", opt)
		}
		newOptions = append(newOptions, opt)
	}

	if !foundWrite {
		newOptions = append(newOptions, "rw")
	}
	if !foundProp {
		newOptions = append(newOptions, "rprivate")
	}
	defaults, err := getDefaultMountOptions(sourcePath)
	if err != nil {
		return nil, err
	}
	if !foundExec && defaults.noexec {
		newOptions = append(newOptions, "noexec")
	}
	if !foundSuid && defaults.nosuid {
		newOptions = append(newOptions, "nosuid")
	}
	if !foundDev && defaults.nodev {
		newOptions = append(newOptions, "nodev")
	}
	if isTmpfs && !foundCopyUp {
		newOptions = append(newOptions, "tmpcopyup")
	}
	if !isTmpfs && !foundBind {
		newOptions = append(newOptions, "rbind")
	}

	return newOptions, nil
}
