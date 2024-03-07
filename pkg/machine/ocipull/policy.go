package ocipull

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPolicyJSONPath should be overwritten at build time with the real path to the directory where
// the shipped policy.json file is located. This can either be absolute path or a relative path. If it
// is relative it will be resolved relative to the podman binary and NOT the CWD.
//
// use "-X github.com/containers/podman/v5/pkg/machine/ocipull.DefaultPolicyJSONPath=/somepath" in go ldflags to overwrite this
var DefaultPolicyJSONPath = ""

const policyfile = "policy.json"

type defaultPolicyError struct {
	errs []error
}

func (e *defaultPolicyError) Error() string {
	return fmt.Sprintf("no DefaultPolicyJSONPath defined and no local overwrites found: %q", e.errs)
}

func policyPath() (string, error) {
	paths := localPolicyOverwrites()
	errs := make([]error, 0, len(paths))
	for _, path := range paths {
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
		errs = append(errs, err)
	}
	if DefaultPolicyJSONPath != "" {
		if filepath.IsAbs(DefaultPolicyJSONPath) {
			return filepath.Join(DefaultPolicyJSONPath, policyfile), nil
		}
		p, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("could not resolve relative path to binary: %w", err)
		}
		return filepath.Join(filepath.Dir(p), DefaultPolicyJSONPath, policyfile), nil
	}
	return "", &defaultPolicyError{errs: errs}
}
