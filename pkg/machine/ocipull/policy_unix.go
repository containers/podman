//go:build !windows

package ocipull

import (
	"path/filepath"

	"go.podman.io/common/pkg/config"
	"go.podman.io/storage/pkg/homedir"
)

func localPolicyOverwrites() []string {
	var dirs []string
	if p, err := homedir.GetConfigHome(); err == nil {
		dirs = append(dirs, filepath.Join(p, "containers", policyfile))
	}
	dirs = append(dirs, config.DefaultSignaturePolicyPath)
	return dirs
}
