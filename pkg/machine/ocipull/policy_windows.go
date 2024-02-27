package ocipull

import (
	"os"
	"path/filepath"
)

func localPolicyOverwrites() []string {
	return []string{filepath.Join(os.Getenv("APPDATA"), "containers", policyfile)}
}
