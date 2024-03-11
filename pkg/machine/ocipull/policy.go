package ocipull

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// DefaultPolicyJSONPath should be overwritten at build time with the real path to the directory where
// the shipped policy.json file is located. This can either be absolute path or a relative path. If it
// is relative it will be resolved relative to the podman binary and NOT the CWD.
//
// use "-X github.com/containers/podman/v5/pkg/machine/ocipull.DefaultPolicyJSONPath=/somepath" in go ldflags to overwrite this
var DefaultPolicyJSONPath = ""

const policyfile = "policy.json"

// policyPaths returns a slice of possible directories where a policy.json might live
func policyPaths() []string {
	paths := localPolicyOverwrites()
	if DefaultPolicyJSONPath != "" {
		if filepath.IsAbs(DefaultPolicyJSONPath) {
			return append(paths, filepath.Join(DefaultPolicyJSONPath, policyfile))
		}
		p, err := os.Executable()
		if err != nil {
			logrus.Warnf("could not resolve relative path to binary: %q", err)
		}
		paths = append(paths, filepath.Join(filepath.Dir(p), DefaultPolicyJSONPath, policyfile))
	}
	return paths
}
