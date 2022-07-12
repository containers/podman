package util

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrBadMntOption indicates that an invalid mount option was passed.
	ErrBadMntOption = errors.New("invalid mount option")
	// ErrDupeMntOption indicates that a duplicate mount option was passed.
	ErrDupeMntOption = errors.New("duplicate mount option passed")
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
		foundWrite, foundSize, foundProp, foundMode, foundExec, foundSuid, foundDev, foundCopyUp, foundBind, foundZ, foundU, foundOverlay, foundIdmap, foundCopy bool
	)

	newOptions := make([]string, 0, len(options))
	for _, opt := range options {
		// Some options have parameters - size, mode
		splitOpt := strings.SplitN(opt, "=", 2)

		// add advanced options such as upperdir=/path and workdir=/path, when overlay is specified
		if foundOverlay {
			if strings.Contains(opt, "upperdir") {
				newOptions = append(newOptions, opt)
				continue
			}
			if strings.Contains(opt, "workdir") {
				newOptions = append(newOptions, opt)
				continue
			}
		}

		if strings.HasPrefix(splitOpt[0], "idmap") {
			if foundIdmap {
				return nil, fmt.Errorf("the 'idmap' option can only be set once: %w", ErrDupeMntOption)
			}
			foundIdmap = true
			newOptions = append(newOptions, opt)
			continue
		}

		switch splitOpt[0] {
		case "copy", "nocopy":
			if foundCopy {
				return nil, fmt.Errorf("only one of 'nocopy' and 'copy' can be used: %w", ErrDupeMntOption)
			}
			foundCopy = true
		case "O":
			foundOverlay = true
		case "volume-opt":
			// Volume-opt should be relayed and processed by driver.
			newOptions = append(newOptions, opt)
		case "exec", "noexec":
			if foundExec {
				return nil, fmt.Errorf("only one of 'noexec' and 'exec' can be used: %w", ErrDupeMntOption)
			}
			foundExec = true
		case "suid", "nosuid":
			if foundSuid {
				return nil, fmt.Errorf("only one of 'nosuid' and 'suid' can be used: %w", ErrDupeMntOption)
			}
			foundSuid = true
		case "nodev", "dev":
			if foundDev {
				return nil, fmt.Errorf("only one of 'nodev' and 'dev' can be used: %w", ErrDupeMntOption)
			}
			foundDev = true
		case "rw", "ro":
			if foundWrite {
				return nil, fmt.Errorf("only one of 'rw' and 'ro' can be used: %w", ErrDupeMntOption)
			}
			foundWrite = true
		case "private", "rprivate", "slave", "rslave", "shared", "rshared", "unbindable", "runbindable":
			if foundProp {
				return nil, fmt.Errorf("only one root propagation mode can be used: %w", ErrDupeMntOption)
			}
			foundProp = true
		case "size":
			if !isTmpfs {
				return nil, fmt.Errorf("the 'size' option is only allowed with tmpfs mounts: %w", ErrBadMntOption)
			}
			if foundSize {
				return nil, fmt.Errorf("only one tmpfs size can be specified: %w", ErrDupeMntOption)
			}
			foundSize = true
		case "mode":
			if !isTmpfs {
				return nil, fmt.Errorf("the 'mode' option is only allowed with tmpfs mounts: %w", ErrBadMntOption)
			}
			if foundMode {
				return nil, fmt.Errorf("only one tmpfs mode can be specified: %w", ErrDupeMntOption)
			}
			foundMode = true
		case "tmpcopyup":
			if !isTmpfs {
				return nil, fmt.Errorf("the 'tmpcopyup' option is only allowed with tmpfs mounts: %w", ErrBadMntOption)
			}
			if foundCopyUp {
				return nil, fmt.Errorf("the 'tmpcopyup' or 'notmpcopyup' option can only be set once: %w", ErrDupeMntOption)
			}
			foundCopyUp = true
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		case "notmpcopyup":
			if !isTmpfs {
				return nil, fmt.Errorf("the 'notmpcopyup' option is only allowed with tmpfs mounts: %w", ErrBadMntOption)
			}
			if foundCopyUp {
				return nil, fmt.Errorf("the 'tmpcopyup' or 'notmpcopyup' option can only be set once: %w", ErrDupeMntOption)
			}
			foundCopyUp = true
			// do not propagate notmpcopyup to the OCI runtime
			continue
		case "bind", "rbind":
			if isTmpfs {
				return nil, fmt.Errorf("the 'bind' and 'rbind' options are not allowed with tmpfs mounts: %w", ErrBadMntOption)
			}
			if foundBind {
				return nil, fmt.Errorf("only one of 'rbind' and 'bind' can be used: %w", ErrDupeMntOption)
			}
			foundBind = true
		case "z", "Z":
			if isTmpfs {
				return nil, fmt.Errorf("the 'z' and 'Z' options are not allowed with tmpfs mounts: %w", ErrBadMntOption)
			}
			if foundZ {
				return nil, fmt.Errorf("only one of 'z' and 'Z' can be used: %w", ErrDupeMntOption)
			}
			foundZ = true
		case "U":
			if foundU {
				return nil, fmt.Errorf("the 'U' option can only be set once: %w", ErrDupeMntOption)
			}
			foundU = true
		default:
			return nil, fmt.Errorf("unknown mount option %q: %w", opt, ErrBadMntOption)
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

func ParseDriverOpts(option string) (string, string, error) {
	token := strings.SplitN(option, "=", 2)
	if len(token) != 2 {
		return "", "", fmt.Errorf("cannot parse driver opts: %w", ErrBadMntOption)
	}
	opt := strings.SplitN(token[1], "=", 2)
	if len(opt) != 2 {
		return "", "", fmt.Errorf("cannot parse driver opts: %w", ErrBadMntOption)
	}
	return opt[0], opt[1], nil
}
